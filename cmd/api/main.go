package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/driftguard/driftguard/internal/api/routes"
	"github.com/driftguard/driftguard/internal/collectors/aws"
	"github.com/driftguard/driftguard/internal/config"
	"github.com/driftguard/driftguard/internal/db"
	"github.com/driftguard/driftguard/internal/engine"
	"github.com/driftguard/driftguard/internal/scheduler"
	"go.uber.org/zap"
)

func main() {
	// Logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Database
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	// Run migrations
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// Redis
	redisClient, err := db.ConnectRedis(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	// Drift engine
	driftEngine := engine.New(database, logger)

	// AWS Collector
	awsCollector := aws.NewCollector(cfg, database, driftEngine, logger)

	// Scheduler — poll every 15 minutes
	sched := scheduler.New(logger)
	sched.AddJob("@every 15m", "aws-collector", awsCollector.Collect)
	sched.Start()
	defer sched.Stop()

	// Run first collection immediately on startup
	go func() {
		logger.Info("running initial drift collection")
		ctx := context.Background()
		if err := awsCollector.Collect(ctx); err != nil {
			logger.Error("initial collection failed", zap.Error(err))
		}
	}()

	// HTTP server
	router := routes.Setup(cfg, database, redisClient, driftEngine, logger)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		logger.Info("server starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}
	logger.Info("server exited")
}
