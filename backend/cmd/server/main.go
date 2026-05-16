package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mvp-push-gateway/backend/internal/audit"
	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/db"
	"mvp-push-gateway/backend/internal/delivery"
	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/matchgroup"
	"mvp-push-gateway/backend/internal/messagelog"
	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/planning"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/recipient"
	"mvp-push-gateway/backend/internal/route"
	appruntime "mvp-push-gateway/backend/internal/runtime"
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
	if cfg.Postgres.DSN != "" {
		var err error
		pools, err = db.OpenPools(ctx, cfg.Postgres)
		if err != nil {
			log.Fatalf("database connection failed: %v", err)
		}
		defer pools.Close()

		repository := db.NewRepository(pools.API)
		if err := repository.Ping(ctx); err != nil {
			log.Fatalf("database ping failed: %v", err)
		}
		authService := auth.NewService(repository)
		settingsService := settings.NewService(repository)
		if err := settingsService.EnsureDefaults(ctx); err != nil {
			log.Fatalf("seed system settings failed: %v", err)
		}
		sourceService := source.NewService(
			repository,
			source.WithMaxPayloadSizeFunc(func(ctx context.Context) int64 {
				return int64(settingsService.IntSetting(ctx, settings.KeyIngestMaxPayloadBytes, int(settings.DefaultIngestMaxPayloadBytes)))
			}),
		)
		providerService := provider.NewService(repository)
		if err := providerService.SeedProviderCapabilities(ctx); err != nil {
			log.Fatalf("seed provider capabilities failed: %v", err)
		}
		recipientService := recipient.NewService(repository)
		routeService := route.NewService(repository)
		templateService := msgtemplate.NewService(repository)
		matchGroupService := matchgroup.NewService(repository)
		messageLogService := messagelog.NewService(repository)
		auditService := audit.NewService(repository)
		monitoringService := monitoring.NewService(
			db.NewRepository(pools.API),
			db.NewRepository(pools.Maintenance),
		)
		statisticsService := statistics.NewService(db.NewRepository(pools.API))
		workerHarness = appruntime.NewHarness(appruntime.Config{
			PlanningWorker: planning.NewWorker(db.NewRepository(pools.Planning)),
			DeliveryWorker: delivery.NewWorker(db.NewRepository(pools.Sending)),
			DeliveryBatchSizeFunc: func(ctx context.Context) int {
				return settingsService.IntSetting(ctx, settings.KeyRuntimeDeliveryConcurrency, settings.DefaultDeliveryGlobalConcurrency)
			},
			Recovery:         db.NewRepository(pools.Maintenance),
			RetentionCleaner: monitoringService,
		})
		handlerOptions = append(
			handlerOptions,
			httpapi.WithAuthService(authService),
			httpapi.WithSourceService(sourceService),
			httpapi.WithProviderService(providerService),
			httpapi.WithRecipientService(recipientService),
			httpapi.WithRouteService(routeService),
			httpapi.WithTemplateService(templateService),
			httpapi.WithMonitoringService(monitoringService),
			httpapi.WithStatisticsService(statisticsService),
			httpapi.WithMatchGroupService(matchGroupService),
			httpapi.WithMessageLogService(messageLogService),
			httpapi.WithAuditService(auditService),
			httpapi.WithSettingsService(settingsService),
		)
	}

	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Server.Host, cfg.Server.Port),
		Handler:           httpapi.NewHandler(cfg, handlerOptions...),
		ReadHeaderTimeout: 5 * time.Second,
	}

	runtimeCtx, stopRuntime := context.WithCancel(context.Background())
	if workerHarness != nil {
		workerHarness.Start(runtimeCtx)
	}

	go func() {
		log.Printf("%s listening on %s", cfg.App.Name, server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	stopRuntime()
	if workerHarness != nil {
		if err := workerHarness.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("worker shutdown failed: %v", err)
		}
	}
}
