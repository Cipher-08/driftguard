package routes

import (
	"time"

	"github.com/driftguard/driftguard/internal/api/handlers"
	"github.com/driftguard/driftguard/internal/api/middleware"
	"github.com/driftguard/driftguard/internal/config"
	"github.com/driftguard/driftguard/internal/engine"
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

	h := handlers.New(cfg, db, eng, logger)

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

		// Resources
		api.GET("/resources", h.ListResources)

		// Drift
		api.GET("/drifts", h.ListDrifts)
		api.PATCH("/drifts/:id/resolve", h.ResolveDrift)

			   // Remediation
		api.POST("/drifts/:drift_id/remediation/:rem_id/pr", h.OpenPR)
	}

	return r
}
