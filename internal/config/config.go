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

	// AI remediation — free, pluggable providers. The first one with
	// credentials is used (order: Groq, Gemini, Ollama).
	LLMProvider  string // optional explicit override: groq|gemini|ollama
	GroqAPIKey   string
	GroqModel    string
	GeminiAPIKey string
	GeminiModel  string
	OllamaHost   string
	OllamaModel  string
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
		LLMProvider:         getEnv("LLM_PROVIDER", ""),
		GroqAPIKey:          getEnv("GROQ_API_KEY", ""),
		GroqModel:           getEnv("GROQ_MODEL", ""),
		GeminiAPIKey:        getEnv("GEMINI_API_KEY", ""),
		GeminiModel:         getEnv("GEMINI_MODEL", ""),
		OllamaHost:          getEnv("OLLAMA_HOST", ""),
		OllamaModel:         getEnv("OLLAMA_MODEL", ""),
	}

	if cfg.JWTSecret == "change-me-in-production-please" && cfg.Environment == "production" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}

	return cfg, nil
}

// LLMProviderName reports which AI provider will be used, or "none".
func (c *Config) LLMProviderName() string {
	switch {
	case c.LLMProvider == "groq" && c.GroqAPIKey != "":
		return "groq"
	case c.LLMProvider == "gemini" && c.GeminiAPIKey != "":
		return "gemini"
	case c.LLMProvider == "ollama" && c.OllamaHost != "":
		return "ollama"
	case c.GroqAPIKey != "":
		return "groq"
	case c.GeminiAPIKey != "":
		return "gemini"
	case c.OllamaHost != "":
		return "ollama"
	}
	return "none"
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
