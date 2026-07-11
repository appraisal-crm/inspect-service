// @title           Inspect Service API
// @version         1.0
// @description     API for managing property inspections
// @host            localhost:8082
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and your token

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	_ "github.com/appraisal-crm/inspect-service/api"
	"github.com/appraisal-crm/inspect-service/config"
	"github.com/appraisal-crm/inspect-service/internal/handler"
	"github.com/appraisal-crm/inspect-service/internal/outbox"
	"github.com/appraisal-crm/inspect-service/internal/repository"
	"github.com/appraisal-crm/inspect-service/internal/service"
	"github.com/appraisal-crm/inspect-service/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Structured JSON logs on stdout.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		slog.Error("database is not reachable", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// JWKS keys for JWT validation are fetched (and refreshed) from Keycloak.
	jwks, err := keyfunc.NewDefault([]string{cfg.JWKSUrl})
	if err != nil {
		slog.Error("failed to initialize JWKS", "error", err)
		os.Exit(1)
	}
	slog.Info("JWKS initialized", "url", cfg.JWKSUrl)

	// Wire the dependency chain: repo + storage → service → router.
	store := storage.NewStubStorage(cfg.S3Endpoint, cfg.S3Bucket)
	repo := repository.NewPostgresRepository(db)
	svc := service.NewInspectionService(repo, store)
	router := handler.NewRouter(svc, jwks, strings.Split(cfg.AllowedOrigins, ","))

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Outbox relay: publishes inspect.completed events written by the service.
	producer := outbox.NewProducer(strings.Split(cfg.KafkaBrokers, ","))
	defer producer.Close()
	relay := outbox.NewRelay(db, producer, cfg.OutboxPollInterval)
	relayDone := make(chan struct{})
	go func() {
		relay.Run(ctx)
		close(relayDone)
	}()
	slog.Info("outbox relay started", "interval", cfg.OutboxPollInterval)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		slog.Error("server error", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
		// ctx is already cancelled, so the relay loop is exiting — wait for it.
		<-relayDone
		slog.Info("outbox relay stopped")
		slog.Info("server stopped")
	}
}
