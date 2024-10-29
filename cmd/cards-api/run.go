package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	kitlog "github.com/demo-org-np-migration/cauri-go-kit/log"
	"github.com/demo-org-np-migration/cauri-go-kit/httpx"

	httpapi "github.com/demo-org-np-migration/cards-api/internal/http"
	"github.com/demo-org-np-migration/cards-api/internal/ledger"
	"github.com/demo-org-np-migration/cards-api/internal/processor"
	"github.com/demo-org-np-migration/cards-api/internal/store"
)

func run() {
	serviceName := getEnv("SERVICE_NAME", "cards-api")
	env := getEnv("ENV", "staging")
	port := getEnv("PORT", "8080")
	migrationsPath := getEnv("MIGRATIONS_PATH", "file://migrations")

	databaseURL := os.Getenv("DATABASE_URL")
	keycloakIssuer := os.Getenv("KEYCLOAK_ISSUER")
	keycloakClientID := getEnv("KEYCLOAK_CLIENT_ID", "cards-api")
	keycloakClientSecret := os.Getenv("KEYCLOAK_CLIENT_SECRET")
	ledgerCoreURL := os.Getenv("LEDGER_CORE_URL")
	cardnetURL := os.Getenv("CARDNET_URL")
	cardnetAPIKey := os.Getenv("CARDNET_API_KEY")

	logger := kitlog.New(serviceName, env)

	for name, value := range map[string]string{
		"DATABASE_URL":           databaseURL,
		"KEYCLOAK_ISSUER":        keycloakIssuer,
		"KEYCLOAK_CLIENT_SECRET": keycloakClientSecret,
		"LEDGER_CORE_URL":        ledgerCoreURL,
		"CARDNET_URL":            cardnetURL,
		"CARDNET_API_KEY":        cardnetAPIKey,
	} {
		if value == "" {
			logger.Error(name + " is required")
			os.Exit(1)
		}
	}

	pool, err := connectWithRetry(logger, databaseURL, 10, 2*time.Second)
	if err != nil {
		logger.Error("could not connect to cards database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := store.RunMigrations(databaseURL, migrationsPath); err != nil {
		logger.Error("could not run migrations", "err", err)
		os.Exit(1)
	}

	pg := store.NewPostgres(pool)

	tokenProvider := httpx.NewClientCredentials(keycloakIssuer, keycloakClientID, keycloakClientSecret)
	ledgerClient := ledger.NewLedgerClient(ledgerCoreURL, tokenProvider)
	cardnetClient := processor.NewCardnetClient(cardnetURL, cardnetAPIKey)

	router := httpapi.NewRouter(pg, cardnetClient, ledgerClient, logger, keycloakIssuer)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		logger.Info("listening", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	logger.Info("shutting down")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// connectWithRetry retries store.Connect up to attempts times. Without
// this, cards-api starting alongside a Postgres that's still doing initdb
// (docker-compose's depends_on doesn't wait for readiness, and neither does
// a freshly-rolled pod in k8s without an initContainer) means the very
// first Ping fails and the process exits before Postgres ever gets a
// chance to come up.
func connectWithRetry(logger *slog.Logger, databaseURL string, attempts int, backoff time.Duration) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := store.Connect(ctx, databaseURL)
		cancel()
		if err == nil {
			return pool, nil
		}
		lastErr = err
		logger.Warn("cards database not reachable yet, retrying", "attempt", i, "attempts", attempts, "err", err)
		time.Sleep(backoff)
	}
	return nil, lastErr
}
