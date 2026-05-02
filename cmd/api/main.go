package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	vk "github.com/valkey-io/valkey-go"

	pgAdapter "github.com/localpull/orders/internal/adapters/postgres"
	"github.com/localpull/orders/internal/adapters/projection"
	vkAdapter "github.com/localpull/orders/internal/adapters/valkey"
	"github.com/localpull/orders/internal/order"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := loadConfig()

	// --- Infrastructure ---

	pgPool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pgPool.Close()

	valkeyClient, err := vk.NewClient(vk.ClientOption{
		InitAddress: []string{cfg.ValkeyAddr},
	})
	if err != nil {
		return fmt.Errorf("valkey: %w", err)
	}
	defer valkeyClient.Close()

	// gochannel is in-process and zero-config — fine for local dev and tests.
	// For production swap with NATS, Kafka, or watermill-sql (PostgreSQL Pub/Sub).
	wmLogger := watermill.NewStdLogger(false, false)
	pubSub := gochannel.NewGoChannel(gochannel.Config{}, wmLogger)

	router, err := message.NewRouter(message.RouterConfig{}, wmLogger)
	if err != nil {
		return fmt.Errorf("watermill router: %w", err)
	}

	// --- Adapters (implement domain ports) ---

	writeRepo := pgAdapter.NewOrderWriteRepo(pgPool)

	pgReadRepo := pgAdapter.NewOrderReadRepo(pgPool)
	cachedReadRepo := vkAdapter.NewOrderReadRepo(pgReadRepo, valkeyClient)

	// The projector invalidates the Valkey cache when an order is created.
	proj := projection.NewOrderProjector(cachedReadRepo)
	router.AddNoPublisherHandler(
		"order_cache_invalidation",
		"orders.created",
		pubSub,
		proj.Handler,
	)

	outboxRelay := pgAdapter.NewOutboxRelay(pgPool, pubSub)

	// --- Vertical slices (depend only on domain ports, not concrete adapters) ---

	ordersModule := order.NewModule(writeRepo, cachedReadRepo)

	// --- HTTP ---

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	ordersModule.Mount(r)

	// --- Run ---

	errCh := make(chan error, 2)

	go func() { errCh <- router.Run(ctx) }()
	go func() { errCh <- outboxRelay.Run(ctx) }()

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down gracefully")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

type config struct {
	PostgresDSN string
	ValkeyAddr  string
	HTTPAddr    string
}

func loadConfig() config {
	return config{
		PostgresDSN: envOr("POSTGRES_DSN", "postgresql://orders:orders@localhost:5432/orders"),
		ValkeyAddr:  envOr("VALKEY_ADDR", "localhost:6379"),
		HTTPAddr:    envOr("HTTP_ADDR", ":8080"),
	}
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
