package routes

import (
	"time"

	"github.com/driftguard/driftguard/internal/api/handlers"
	"github.com/driftguard/driftguard/internal/api/middleware"
	"github.com/driftguard/driftguard/internal/config"
	"github.com/driftguard/driftguard/internal/engine"
	"github.com/driftguard/driftguard/internal/llm"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func Setup(
	cfg *config.Config,
	db *pgxpool.Pool,
	_ *redis.Client,
	eng *engine.Engine,
	llmClient llm.Client,
	runScan handlers.ScanFunc,
	logger *zap.Logger,
) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	h := handlers.New(cfg, db, eng, llmClient, runScan, logger)

	// Public routes
	r.GET("/health", h.Health)
	r.POST("/api/v1/auth/register", h.Register)
	r.POST("/api/v1/auth/login", h.Login)

	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.Auth(cfg.JWTSecret))
	{
		// Dashboard
		api.GET("/summary", h.GetSummary)

		// Cloud Credentials
		api.POST("/credentials", h.AddCredential)
		api.GET("/credentials", h.ListCredentials)
		api.DELETE("/credentials/:id", h.DeleteCredential)

		// On-demand scan
		api.POST("/scan", h.TriggerScan)

		// Resources
		api.GET("/resources", h.ListResources)

		// Drift (consistent :id param throughout to avoid Gin wildcard conflicts)
		api.GET("/drifts", h.ListDrifts)
		api.GET("/drifts/:id", h.GetDrift)
		api.PATCH("/drifts/:id/resolve", h.ResolveDrift)

		// Remediation
		api.POST("/drifts/:id/remediation", h.GenerateRemediation)
		api.POST("/drifts/:id/remediation/:rem_id/pr", h.OpenPR)
	}

	return r
}
