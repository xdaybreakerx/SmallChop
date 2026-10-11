package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gochop-it/internal/config"
	"gochop-it/internal/handlers"
	"gochop-it/internal/observability"
	"gochop-it/internal/repository"
	"gochop-it/internal/routes"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	observer := observability.New(logger)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid application configuration", "error", err.Error())
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeouts.Startup)

	// MongoDB setup
	mongoRepo, err := repository.NewMongoRepo(ctx)
	cancel()
	if err != nil {
		logger.Error("required MongoDB startup failed")
		return err
	}
	logger.Info("MongoDB connected")

	// Redis is optional; keep the client so later requests can recover automatically.
	var cache handlers.URLCache
	var redisRepo *repository.RedisRepo
	if cfg.CacheEnabled {
		redisRepo = repository.NewRedisRepo(cfg.Timeouts.Cache)
		cache = redisRepo
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), cfg.Timeouts.Cache)
		if err := redisRepo.Ping(cacheCtx); err != nil {
			logger.Warn("Redis unavailable at startup; using MongoDB fallback")
		}
		cacheCancel()
	}
	defer func() {
		if redisRepo != nil {
			if err := redisRepo.Client.Close(); err != nil {
				logger.Warn("Redis close failed")
			}
		}
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
		defer closeCancel()
		if err := mongoRepo.Client.Disconnect(closeCtx); err != nil {
			logger.Warn("MongoDB close failed")
		}
	}()

	// Initialize Handlers
	handlers, err := handlers.NewHandlers(mongoRepo, cache, cfg.PublicBaseURL, cfg.Timeouts)
	if err != nil {
		logger.Error("handler initialization failed")
		return err
	}

	handlers.Observer = observer
	handlers.MongoHealth = mongoRepo

	// Register Routes
	mux := routes.NewMux(handlers, cfg)

	// Server Setup
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15*time.Second + cfg.Timeouts.Request,
		IdleTimeout:       60 * time.Second,
	}

	metricsServer := &http.Server{
		Addr: ":9090", Handler: observer.MetricsHandler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	logger.Info("HTTP listeners starting", "application_address", srv.Addr, "metrics_address", metricsServer.Addr)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, srv, metricsServer); err != nil {
		logger.Error("HTTP listener failed")
		return err
	}
	logger.Info("server exited")
	return nil
}

// A listener failure stops both servers; shutdown shares a bounded budget.
func serve(ctx context.Context, servers ...*http.Server) error {
	serverErrors := make(chan error, len(servers))
	for _, server := range servers {
		go func() { serverErrors <- server.ListenAndServe() }()
	}
	var failure error
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != http.ErrServerClosed {
			failure = err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, server := range servers {
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Warn("HTTP shutdown budget exceeded")
			_ = server.Close()
		}
	}
	return failure
}
