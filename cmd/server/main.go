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

	"gochop-it/internal/handlers"
	"gochop-it/internal/repository"
	"gochop-it/internal/routes"
)

func main() {
	ctx := context.Background()

	// MongoDB setup
	mongoRepo, err := repository.NewMongoRepo(ctx)
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %v", err)
	}
	fmt.Println("Connected to MongoDB!")

	// Ensure MongoDB connection is valid
	err = mongoRepo.Client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}
	fmt.Println("MongoDB connection is active!")

	// Redis setup
	redisRepo := repository.NewRedisRepo()

	// Ensure Redis connection is valid
	if e := redisRepo.Ping(ctx); e != nil {
		log.Fatalf("Could not connect to Redis: %v", e)
	}
	fmt.Println("Connected to Redis!")

	// Initialize Handlers
	handlers, err := handlers.NewHandlers(mongoRepo, redisRepo)
	if err != nil {
		log.Fatalf("Failed to initialize handlers: %v", err)
	}

	// Register Routes
	routes.RegisterRoutes(handlers)

	// Server Setup
	srv := &http.Server{
		Addr: ":8080",
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
