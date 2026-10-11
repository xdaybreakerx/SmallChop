package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gochop-it/internal/config"
	"gochop-it/internal/handlers"
	"gochop-it/internal/repository"
	"gochop-it/internal/routes"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Invalid application configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeouts.Startup)

	// MongoDB setup
	mongoRepo, err := repository.NewMongoRepo(ctx)
	cancel()
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %v", err)
	}
	fmt.Println("Connected to MongoDB!")

	// Redis is optional; keep the client so later requests can recover automatically.
	var cache handlers.URLCache
	var redisRepo *repository.RedisRepo
	if cfg.CacheEnabled {
		redisRepo = repository.NewRedisRepo(cfg.Timeouts.Cache)
		cache = redisRepo
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), cfg.Timeouts.Cache)
		if err := redisRepo.Ping(cacheCtx); err != nil {
			log.Println("Redis unavailable at startup; using MongoDB fallback")
		}
		cacheCancel()
	}
	defer func() {
		if redisRepo != nil {
			if err := redisRepo.Client.Close(); err != nil {
				log.Printf("Redis close failed: %v", err)
			}
		}
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
		defer closeCancel()
		if err := mongoRepo.Client.Disconnect(closeCtx); err != nil {
			log.Printf("MongoDB close failed: %v", err)
		}
	}()

	// Initialize Handlers
	handlers, err := handlers.NewHandlers(mongoRepo, cache, cfg.PublicBaseURL, cfg.Timeouts)
	if err != nil {
		log.Fatalf("Failed to initialize handlers: %v", err)
	}

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

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()
	fmt.Println("Server is running on http://localhost:8080")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down server...")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutDown); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	fmt.Println("Server exiting")
}
