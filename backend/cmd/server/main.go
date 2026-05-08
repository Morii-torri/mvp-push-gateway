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

	"mvp-push-gateway/backend/internal/config"
	httpapi "mvp-push-gateway/backend/internal/http"
)

func main() {
	cfg := config.Load()
	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Server.Host, cfg.Server.Port),
		Handler:           httpapi.NewHandler(cfg),
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
