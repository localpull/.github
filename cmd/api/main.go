package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	vk "github.com/valkey-io/valkey-go"
	"golang.org/x/sync/errgroup"

	pgAdapter "github.com/localpull/orders/internal/adapters/postgres"
	"github.com/localpull/orders/internal/adapters/projection"
	vkAdapter "github.com/localpull/orders/internal/adapters/valkey"
	"github.com/localpull/orders/internal/order"
)

type contextKey string

const reqIDKey contextKey = "request_id"

func main() {
	slog.SetDefault(newLogger())

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

	poolCfg, err := pgxpool.ParseConfig(cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("pgxpool parse config: %w", err)
	}
	poolCfg.MaxConns = int32(cfg.PGMaxConns)
	pgPool, err := pgxpool.NewWithConfig(ctx, poolCfg)
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

	proj := projection.NewOrderProjector(cachedReadRepo)
	router.AddNoPublisherHandler(
		"order_cache_invalidation",
		"orders.created",
		pubSub,
		proj.Handler,
	)

	outboxRelay := pgAdapter.NewOutboxRelay(pgPool, pubSub)

	// --- HTTP ---

	mux := http.NewServeMux()
	order.NewModule(writeRepo, cachedReadRepo).Register(mux)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      withRequestID(withLogging(withRecovery(mux))),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- Run: errgroup gives each component the same derived context so any
	// failure cancels all components, and resources are only closed after all
	// goroutines exit (pool/client defers run when run() returns from g.Wait).

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := router.Run(gCtx); !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	g.Go(func() error { return outboxRelay.Run(gCtx) })
	g.Go(func() error {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	// Dedicated goroutine shuts down the HTTP server whenever gCtx is done —
	// whether from a signal or from any other component failing.
	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("shutting down gracefully")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Warn("http server shutdown incomplete", "err", err)
		}
		return nil
	})

	return g.Wait()
}

// withRequestID generates or propagates X-Request-ID and injects it into the
// request context so downstream middleware and handlers can correlate log lines.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), reqIDKey, id)))
	})
}

// withLogging logs method, path, status, latency, and request_id for every request.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		reqID, _ := r.Context().Value(reqIDKey).(string)
		slog.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
		)
	})
}

// withRecovery converts panics into 500 responses and logs the stack trace.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.ErrorContext(r.Context(), "panic recovered",
					"err", v,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter captures the status code written by handlers.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write ensures the status is recorded even when handlers call Write without
// an explicit WriteHeader (Go's default is 200 in that case).
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

// newLogger returns a JSON logger for production.
// Set LOG_LEVEL=debug to enable debug output.
func newLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if os.Getenv("LOG_LEVEL") == "debug" {
		opts.Level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

type config struct {
	PostgresDSN string
	ValkeyAddr  string
	HTTPAddr    string
	PGMaxConns  int
}

func loadConfig() config {
	return config{
		PostgresDSN: requireEnv("POSTGRES_DSN"),
		ValkeyAddr:  envOr("VALKEY_ADDR", "localhost:6379"),
		HTTPAddr:    envOr("HTTP_ADDR", ":8080"),
		PGMaxConns:  envInt("PG_MAX_CONNS", 25),
	}
}

func requireEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		slog.Error("required environment variable not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		slog.Warn("invalid env value, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return n
}
