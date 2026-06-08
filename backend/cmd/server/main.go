package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mvp-push-gateway/backend/internal/audit"
	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/db"
	"mvp-push-gateway/backend/internal/deadletter"
	"mvp-push-gateway/backend/internal/delivery"
	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/matchgroup"
	"mvp-push-gateway/backend/internal/messagelog"
	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/planning"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
	"mvp-push-gateway/backend/internal/recipient"
	"mvp-push-gateway/backend/internal/route"
	appruntime "mvp-push-gateway/backend/internal/runtime"
	"mvp-push-gateway/backend/internal/secretbox"
	"mvp-push-gateway/backend/internal/settings"
	"mvp-push-gateway/backend/internal/source"
	"mvp-push-gateway/backend/internal/statistics"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	var handlerOptions []httpapi.Option
	var pools *db.PoolSet
	var workerHarness *appruntime.Harness
	var startJetStreamConsumers func(context.Context)
	secretCipher, err := secretbox.NewCipherFromBase64(cfg.Security.SecretEncryptionKeyID, cfg.Security.SecretEncryptionKey)
	if err != nil {
		log.Fatalf("secret encryption key invalid: %v", err)
	}
	repositoryOptions := []db.RepositoryOption{db.WithSecretCipher(secretCipher)}
	if cfg.Postgres.DSN != "" {
		pools, err = db.OpenPools(ctx, cfg.Postgres)
		if err != nil {
			log.Fatalf("database connection failed: %v", err)
		}
		defer pools.Close()
		asyncRuntimeLogs := db.NewAsyncRuntimeLogWriter(pools.Maintenance)
		defer func() {
			closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer closeCancel()
			if err := asyncRuntimeLogs.Close(closeCtx); err != nil {
				log.Printf("async runtime log writer close failed: %v", err)
			}
		}()

		repository := db.NewRepository(pools.API, repositoryOptions...)
		if err := repository.Ping(ctx); err != nil {
			log.Fatalf("database ping failed: %v", err)
		}
		authService := auth.NewService(repository)
		settingsService := settings.NewService(repository)
		if err := settingsService.EnsureDefaults(ctx); err != nil {
			log.Fatalf("seed system settings failed: %v", err)
		}
		var broker queue.Broker
		var natsPublisher *queue.NATSPublisher
		if cfg.Queue.Backend == "jetstream" {
			natsPublisher, err = queue.NewNATSPublisher(ctx, queue.NATSOptions{
				URL:                   cfg.Queue.NATS.URL,
				CredsPath:             cfg.Queue.NATS.CredsPath,
				StreamReplicas:        cfg.Queue.NATS.StreamReplicas,
				LatestPayloadKVBucket: cfg.Queue.NATS.LatestPayloadKVBucket,
				InboundDedupeKVPrefix: cfg.Queue.NATS.InboundDedupeKVPrefix,
				HMACNonceKVPrefix:     cfg.Queue.NATS.HMACNonceKVPrefix,
			})
			if err != nil {
				log.Fatalf("nats jetstream connection failed: %v", err)
			}
			defer natsPublisher.Close()
			broker = queue.NewJetStreamBroker(natsPublisher)
		}
		sourceOptions := []source.Option{
			source.WithMaxPayloadSizeFunc(func(ctx context.Context) int64 {
				return int64(settingsService.IntSetting(ctx, settings.KeyIngestMaxPayloadBytes, int(settings.DefaultIngestMaxPayloadBytes)))
			}),
		}
		if broker != nil {
			sourceOptions = append(sourceOptions, source.WithRoutePlanPublisher(broker))
		}
		if natsPublisher != nil {
			sourceOptions = append(
				sourceOptions,
				source.WithLatestPayloadStore(natsPublisher),
				source.WithInboundDedupeStore(natsPublisher),
				source.WithHMACNonceStore(natsPublisher),
			)
		}
		sourceService := source.NewService(repository, sourceOptions...)
		providerService := provider.NewService(repository)
		if err := providerService.SeedProviderCapabilities(ctx); err != nil {
			log.Fatalf("seed provider capabilities failed: %v", err)
		}
		recipientService := recipient.NewService(repository)
		routeOptions := []route.Option{}
		if natsPublisher != nil {
			routeOptions = append(routeOptions, route.WithChangePublisher(natsPublisher))
		}
		routeService := route.NewService(repository, routeOptions...)
		templateService := msgtemplate.NewService(repository)
		matchGroupService := matchgroup.NewService(repository)
		messageLogService := messagelog.NewService(repository)
		deadLetterService := deadletter.NewService(repository)
		auditService := audit.NewService(repository)
		monitoringOptions := []monitoring.Option{}
		if natsPublisher != nil {
			monitoringOptions = append(monitoringOptions, monitoring.WithJetStreamStatsProvider(natsPublisher))
		}
		monitoringService := monitoring.NewService(
			db.NewRepository(pools.API, repositoryOptions...),
			db.NewRepository(pools.Maintenance, repositoryOptions...),
			monitoringOptions...,
		)
		statisticsService := statistics.NewService(db.NewRepository(pools.API, repositoryOptions...))
		planningOptions := []planning.WorkerOption{}
		deliveryOptions := []delivery.WorkerOption{}
		if broker != nil {
			planningOptions = append(planningOptions, planning.WithSendPublisher(broker))
			deliveryOptions = append(deliveryOptions, delivery.WithResultPublisher(delivery.NewQueueResultPublisher(broker)))
		}
		planningWorker := planning.NewWorker(db.NewRepositoryWithAsyncRuntimeLogWriter(pools.Planning, asyncRuntimeLogs, repositoryOptions...), planningOptions...)
		deliveryWorker := delivery.NewWorker(db.NewRepositoryWithAsyncRuntimeLogWriter(pools.Sending, asyncRuntimeLogs, repositoryOptions...), deliveryOptions...)
		if broker != nil {
			resultQueueWorker := delivery.NewResultQueueWorker(
				broker,
				delivery.NewResultWriter(db.NewRepositoryWithAsyncRuntimeLogWriter(pools.Sending, asyncRuntimeLogs, repositoryOptions...)),
			)
			startJetStreamConsumers = func(ctx context.Context) {
				startConsumerGroup(ctx, "route-plan", cfg.Queue.NATS.RouteConsumers, func(ctx context.Context) error {
					return broker.SubscribeRoutePlan(ctx, planningWorker.ProcessRoutePlanMessage)
				})
				startConsumerGroup(ctx, "send-message", cfg.Queue.NATS.SendConsumers, func(ctx context.Context) error {
					return broker.SubscribeSend(ctx, deliveryWorker.ProcessSendMessage)
				})
				startConsumerGroup(ctx, "result-writer", cfg.Queue.NATS.ResultConsumers, resultQueueWorker.Run)
			}
		}
		routeRuntimeRepository := db.NewRepository(pools.Maintenance, repositoryOptions...)
		var routePlanChangeListener appruntime.RoutePlanChangeListener = routeRuntimeRepository
		if natsPublisher != nil {
			routePlanChangeListener = natsPublisher
		}
		var planningBatchWorker appruntime.BatchWorker = planningWorker
		var deliveryBatchWorker appruntime.BatchWorker = deliveryWorker
		if broker != nil {
			planningBatchWorker = nil
			deliveryBatchWorker = nil
		}
		workerHarness = appruntime.NewHarness(appruntime.Config{
			PlanningWorker:          planningBatchWorker,
			DeliveryWorker:          deliveryBatchWorker,
			RoutePlanCache:          planningWorker,
			RoutePlanSourceLister:   routeRuntimeRepository,
			RoutePlanChangeListener: routePlanChangeListener,
			DeliveryBatchSizeFunc: func(ctx context.Context) int {
				return settingsService.IntSetting(ctx, settings.KeyRuntimeDeliveryConcurrency, settings.DefaultDeliveryGlobalConcurrency)
			},
			Recovery:         db.NewRepository(pools.Maintenance, repositoryOptions...),
			RetentionCleaner: monitoringService,
			RetentionDaysFunc: func(ctx context.Context) int {
				return settingsService.IntSetting(ctx, settings.KeyLogsRetentionDays, monitoring.DefaultRetentionDays)
			},
		})
		handlerOptions = append(
			handlerOptions,
			httpapi.WithAuthService(authService),
			httpapi.WithSourceService(sourceService),
			httpapi.WithProviderService(providerService),
			httpapi.WithRecipientService(recipientService),
			httpapi.WithRouteService(routeService),
			httpapi.WithPlanningWorker(planningWorker),
			httpapi.WithDeliveryWorker(deliveryWorker),
			httpapi.WithRuntimeWorkerPauseController(workerHarness),
			httpapi.WithTemplateService(templateService),
			httpapi.WithMonitoringService(monitoringService),
			httpapi.WithStatisticsService(statisticsService),
			httpapi.WithMatchGroupService(matchGroupService),
			httpapi.WithMessageLogService(messageLogService),
			httpapi.WithDeadLetterService(deadLetterService),
			httpapi.WithAuditService(auditService),
			httpapi.WithSettingsService(settingsService),
		)
	}

	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Server.Host, cfg.Server.Port),
		Handler:           httpapi.NewHandler(cfg, handlerOptions...),
		ReadHeaderTimeout: 5 * time.Second,
	}
	var pprofServer *http.Server
	if cfg.Server.PprofPort != "" {
		pprofServer = &http.Server{
			Addr:              net.JoinHostPort("127.0.0.1", cfg.Server.PprofPort),
			Handler:           newPprofMux(),
			ReadHeaderTimeout: 5 * time.Second,
		}
	}

	runtimeCtx, stopRuntime := context.WithCancel(context.Background())
	if startJetStreamConsumers != nil {
		startJetStreamConsumers(runtimeCtx)
	}
	if workerHarness != nil {
		workerHarness.Start(runtimeCtx)
	}

	go func() {
		log.Printf("%s listening on %s", cfg.App.Name, server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()
	if pprofServer != nil {
		go func() {
			log.Printf("pprof listening on %s", pprofServer.Addr)
			if err := pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("pprof server failed: %v", err)
			}
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	if pprofServer != nil {
		if err := pprofServer.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("pprof server shutdown failed: %v", err)
		}
	}
	stopRuntime()
	if workerHarness != nil {
		if err := workerHarness.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("worker shutdown failed: %v", err)
		}
	}
}

func newPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	return mux
}

func startConsumerGroup(ctx context.Context, name string, count int, run func(context.Context) error) {
	if count <= 0 {
		count = 1
	}
	for index := 0; index < count; index++ {
		index := index
		go func() {
			restartDelay := 500 * time.Millisecond
			for {
				if err := ctx.Err(); err != nil {
					return
				}
				err := run(ctx)
				if errors.Is(err, context.Canceled) {
					return
				}
				if err != nil {
					log.Printf("jetstream consumer %s-%d stopped: %v; restarting in %s", name, index+1, err, restartDelay)
				} else {
					log.Printf("jetstream consumer %s-%d stopped without error; restarting in %s", name, index+1, restartDelay)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(restartDelay):
				}
				if restartDelay < 5*time.Second {
					restartDelay *= 2
					if restartDelay > 5*time.Second {
						restartDelay = 5 * time.Second
					}
				}
			}
		}()
	}
}
