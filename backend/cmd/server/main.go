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

	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/db"
	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/source"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	var handlerOptions []httpapi.Option
	var pools *db.PoolSet
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
		sourceService := source.NewService(repository)
		handlerOptions = append(
			handlerOptions,
			httpapi.WithAuthService(authService),
			httpapi.WithSourceService(sourceService),
		)
	}

	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Server.Host, cfg.Server.Port),
		Handler:           httpapi.NewHandler(cfg, handlerOptions...),
		ReadHeaderTimeout: 5 * time.Second,
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
}
