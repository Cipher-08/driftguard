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
	"github.com/driftguard/driftguard/internal/collectors/gcp"
	"github.com/driftguard/driftguard/internal/compliance"
	"github.com/driftguard/driftguard/internal/config"
	"github.com/driftguard/driftguard/internal/db"
	"github.com/driftguard/driftguard/internal/engine"
	"github.com/driftguard/driftguard/internal/llm"
	"github.com/driftguard/driftguard/internal/scheduler"
	"go.uber.org/zap"
)

// collector is the common interface for cloud resource collectors.
type collector interface {
	Collect(ctx context.Context) error
}

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

	// Compliance checker (built-in CIS/SOC2-style rules)
	checker := compliance.New()

	// Drift engine
	driftEngine := engine.New(database, checker, logger)

	// AI remediation provider (free: Groq / Gemini / Ollama). nil if none configured.
	llmClient := llm.New(llm.Settings{
		Provider:     cfg.LLMProvider,
		GroqAPIKey:   cfg.GroqAPIKey,
		GroqModel:    cfg.GroqModel,
		GeminiAPIKey: cfg.GeminiAPIKey,
		GeminiModel:  cfg.GeminiModel,
		OllamaHost:   cfg.OllamaHost,
		OllamaModel:  cfg.OllamaModel,
	})
	logger.Info("AI remediation provider", zap.String("provider", cfg.LLMProviderName()))

	// Collectors
	collectors := []collector{
		aws.NewCollector(cfg, database, driftEngine, logger),
		gcp.NewCollector(database, driftEngine, logger),
	}

	// runScan executes every collector once. Shared by the scheduler and the
	// on-demand /api/v1/scan endpoint.
	runScan := func(ctx context.Context) error {
		for _, col := range collectors {
			if err := col.Collect(ctx); err != nil {
				logger.Error("collector failed", zap.Error(err))
			}
		}
		return nil
	}

	// Scheduler — poll every 15 minutes
	sched := scheduler.New(logger)
	sched.AddJob("@every 15m", "all-collectors", runScan)
	sched.Start()
	defer sched.Stop()

	// Run first collection immediately on startup
	go func() {
		logger.Info("running initial drift collection")
		if err := runScan(context.Background()); err != nil {
			logger.Error("initial collection failed", zap.Error(err))
		}
	}()

	// HTTP server
	router := routes.Setup(cfg, database, redisClient, driftEngine, llmClient, runScan, logger)
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
