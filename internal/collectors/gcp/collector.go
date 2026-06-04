// Package gcp collects live Google Cloud resource state (Compute instances and
// Cloud Storage buckets) using a stored service-account key. It talks to the GCP
// JSON REST APIs directly via an OAuth2 token source, avoiding the heavy SDK.
package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/driftguard/driftguard/internal/engine"
	"github.com/driftguard/driftguard/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Collector fetches GCP resources using stored credentials.
type Collector struct {
	db     *pgxpool.Pool
	engine *engine.Engine
	logger *zap.Logger
}

func NewCollector(db *pgxpool.Pool, eng *engine.Engine, logger *zap.Logger) *Collector {
	return &Collector{db: db, engine: eng, logger: logger}
}

// Collect scans all orgs with active GCP credentials.
func (c *Collector) Collect(ctx context.Context) error {
	c.logger.Info("starting GCP collection")
	start := time.Now()

	rows, err := c.db.Query(ctx, `
		SELECT org_id, credentials FROM cloud_credentials
		WHERE provider = 'gcp' AND is_active = true
	`)
	if err != nil {
		return fmt.Errorf("querying gcp credentials: %w", err)
	}
	defer rows.Close()

	type credRow struct {
		OrgID uuid.UUID
		Raw   json.RawMessage
	}
	var creds []credRow
	for rows.Next() {
		var cr credRow
		if err := rows.Scan(&cr.OrgID, &cr.Raw); err != nil {
			return fmt.Errorf("scanning gcp credential: %w", err)
		}
		creds = append(creds, cr)
	}

	for _, cr := range creds {
		if err := c.collectForOrg(ctx, cr.OrgID, cr.Raw); err != nil {
			c.logger.Error("gcp collection failed for org", zap.String("org_id", cr.OrgID.String()), zap.Error(err))
		}
	}

	c.logger.Info("GCP collection complete", zap.Duration("duration", time.Since(start)))
	return nil
}

func (c *Collector) collectForOrg(ctx context.Context, orgID uuid.UUID, saKey json.RawMessage) error {
	var meta struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(saKey, &meta); err != nil || meta.ProjectID == "" {
		return fmt.Errorf("service account key missing project_id")
	}

	creds, err := google.CredentialsFromJSON(ctx, saKey, "https://www.googleapis.com/auth/cloud-platform.read-only")
	if err != nil {
		return fmt.Errorf("parsing gcp credentials: %w", err)
	}
	client := &gcpClient{
		http:    &http.Client{Timeout: 30 * time.Second},
		ts:      creds.TokenSource,
		project: meta.ProjectID,
	}

	var resources []*models.Resource

	instances, err := c.collectInstances(ctx, client, orgID)
	if err != nil {
		c.logger.Warn("GCE instance collection failed", zap.Error(err), zap.String("org_id", orgID.String()))
	} else {
		resources = append(resources, instances...)
	}

	buckets, err := c.collectBuckets(ctx, client, orgID)
	if err != nil {
		c.logger.Warn("GCS bucket collection failed", zap.Error(err), zap.String("org_id", orgID.String()))
	} else {
		resources = append(resources, buckets...)
	}

	c.logger.Info("collected GCP resources", zap.Int("count", len(resources)), zap.String("project", client.project))

	for _, r := range resources {
		if err := c.engine.UpsertResource(ctx, r); err != nil {
			c.logger.Error("failed to upsert gcp resource", zap.String("resource_id", r.ResourceID), zap.Error(err))
		}
	}
	return nil
}

func (c *Collector) collectInstances(ctx context.Context, client *gcpClient, orgID uuid.UUID) ([]*models.Resource, error) {
	url := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/aggregated/instances", client.project)
	var resp struct {
		Items map[string]struct {
			Instances []struct {
				ID                string `json:"id"`
				Name              string `json:"name"`
				MachineType       string `json:"machineType"`
				Status            string `json:"status"`
				Zone              string `json:"zone"`
				CanIPForward      bool   `json:"canIpForward"`
				NetworkInterfaces []struct {
					NetworkIP     string `json:"networkIP"`
					AccessConfigs []struct {
						NatIP string `json:"natIP"`
					} `json:"accessConfigs"`
				} `json:"networkInterfaces"`
			} `json:"instances"`
		} `json:"items"`
	}
	if err := client.get(ctx, url, &resp); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var out []*models.Resource
	for _, scope := range resp.Items {
		for _, inst := range scope.Instances {
			publicIP := ""
			privateIP := ""
			if len(inst.NetworkInterfaces) > 0 {
				privateIP = inst.NetworkInterfaces[0].NetworkIP
				for _, ac := range inst.NetworkInterfaces[0].AccessConfigs {
					if ac.NatIP != "" {
						publicIP = ac.NatIP
					}
				}
			}
			live := map[string]interface{}{
				"instance_id":    inst.ID,
				"name":           inst.Name,
				"machine_type":   lastSegment(inst.MachineType),
				"status":         inst.Status,
				"zone":           lastSegment(inst.Zone),
				"can_ip_forward": inst.CanIPForward,
				"private_ip":     privateIP,
				"public_ip":      publicIP,
			}
			stateBytes, _ := json.Marshal(live)
			out = append(out, &models.Resource{
				OrgID: orgID, Provider: "gcp", Region: lastSegment(inst.Zone),
				ResourceType: "gce_instance", ResourceID: inst.Name, ResourceName: inst.Name,
				LiveState: stateBytes, LastScannedAt: now,
			})
		}
	}
	return out, nil
}

func (c *Collector) collectBuckets(ctx context.Context, client *gcpClient, orgID uuid.UUID) ([]*models.Resource, error) {
	url := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b?project=%s", client.project)
	var resp struct {
		Items []struct {
			Name             string `json:"name"`
			Location         string `json:"location"`
			IamConfiguration struct {
				UniformBucketLevelAccess struct {
					Enabled bool `json:"enabled"`
				} `json:"uniformBucketLevelAccess"`
				PublicAccessPrevention string `json:"publicAccessPrevention"`
			} `json:"iamConfiguration"`
		} `json:"items"`
	}
	if err := client.get(ctx, url, &resp); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var out []*models.Resource
	for _, b := range resp.Items {
		public := client.bucketIsPublic(ctx, b.Name)
		live := map[string]interface{}{
			"bucket_name":                 b.Name,
			"location":                    b.Location,
			"uniform_bucket_level_access": b.IamConfiguration.UniformBucketLevelAccess.Enabled,
			"public_access_prevention":    b.IamConfiguration.PublicAccessPrevention,
			"public":                      public,
		}
		stateBytes, _ := json.Marshal(live)
		out = append(out, &models.Resource{
			OrgID: orgID, Provider: "gcp", Region: strings.ToLower(b.Location),
			ResourceType: "gcs_bucket", ResourceID: b.Name, ResourceName: b.Name,
			LiveState: stateBytes, LastScannedAt: now,
		})
	}
	return out, nil
}

// gcpClient wraps authenticated GCP REST calls.
type gcpClient struct {
	http    *http.Client
	ts      oauth2.TokenSource
	project string
}

func (c *gcpClient) token(_ context.Context) (string, error) {
	tok, err := c.ts.Token()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func (c *gcpClient) get(ctx context.Context, url string, out interface{}) error {
	tok, err := c.token(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gcp api %s -> %d: %s", url, resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

// bucketIsPublic best-effort checks the IAM policy for allUsers / allAuthenticatedUsers.
func (c *gcpClient) bucketIsPublic(ctx context.Context, bucket string) bool {
	url := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/iam", bucket)
	var policy struct {
		Bindings []struct {
			Members []string `json:"members"`
		} `json:"bindings"`
	}
	if err := c.get(ctx, url, &policy); err != nil {
		return false
	}
	for _, bind := range policy.Bindings {
		for _, m := range bind.Members {
			if m == "allUsers" || m == "allAuthenticatedUsers" {
				return true
			}
		}
	}
	return false
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
