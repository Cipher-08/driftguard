package gcp

import (
	"context"
)

// Collector is a stub for GCP resource collection
// TODO: Implement actual GCP resource fetching using credentials

type Collector struct {
	// Add fields for credentials/config as needed
}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Collect(ctx context.Context) error {
	// TODO: Use credentials to fetch GCP resources and store in DB
	return nil
}
