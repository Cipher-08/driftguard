package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// Server
	Port        string
	Environment string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// JWT
	JWTSecret string
	JWTExpiry int // hours

	// AWS (can also be set per-org in DB)
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string

	// GCP
	GCPProjectID       string
	GCPCredentialsFile string

	// Azure
	AzureSubscriptionID string
	AzureTenantID       string
	AzureClientID       string
	AzureClientSecret   string

	// GitHub (for PR creation)
	GitHubToken string

	// Anthropic (AI remediation)
	AnthropicAPIKey string
	AnthropicModel  string

	// OPA policies path
	PoliciesPath string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		Environment:         getEnv("ENVIRONMENT", "development"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://driftguard:driftguard@localhost:5432/driftguard?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:           getEnv("JWT_SECRET", "change-me-in-production-please"),
		JWTExpiry:           getEnvInt("JWT_EXPIRY_HOURS", 24),
		AWSRegion:           getEnv("AWS_DEFAULT_REGION", "us-east-1"),
		AWSAccessKeyID:      getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey:  getEnv("AWS_SECRET_ACCESS_KEY", ""),
		GCPProjectID:        getEnv("GCP_PROJECT_ID", ""),
		GCPCredentialsFile:  getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
		AzureSubscriptionID: getEnv("AZURE_SUBSCRIPTION_ID", ""),
		AzureTenantID:       getEnv("AZURE_TENANT_ID", ""),
		AzureClientID:       getEnv("AZURE_CLIENT_ID", ""),
		AzureClientSecret:   getEnv("AZURE_CLIENT_SECRET", ""),
		GitHubToken:         getEnv("GITHUB_TOKEN", ""),
		AnthropicAPIKey:     getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel:      getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-20250514"),
		PoliciesPath:        getEnv("POLICIES_PATH", "./policies"),
	}

	if cfg.JWTSecret == "change-me-in-production-please" && cfg.Environment == "production" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
