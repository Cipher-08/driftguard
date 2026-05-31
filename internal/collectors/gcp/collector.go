package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/driftguard/driftguard/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Collector fetches GCP resources using stored credentials
type Collector struct {
	db *pgxpool.Pool
}

func NewCollector(db *pgxpool.Pool) *Collector {
	return &Collector{db: db}
}

// Collect fetches all active GCP credentials and (stub) prints them
func (c *Collector) Collect(ctx context.Context) error {
	rows, err := c.db.Query(ctx, `SELECT id, credentials FROM cloud_credentials WHERE provider = 'gcp' AND is_active = true`)
	if err != nil {
		return fmt.Errorf("failed to fetch GCP credentials: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var credJSON []byte
		if err := rows.Scan(&id, &credJSON); err != nil {
			continue
		}
		var creds map[string]interface{}
		if err := json.Unmarshal(credJSON, &creds); err != nil {
			continue
		}
		// TODO: Use creds to authenticate with GCP and fetch resources
		fmt.Printf("[GCP Collector] Would fetch resources for credential %s: %v\n", id, creds["client_email"])
	}
	return nil
}
