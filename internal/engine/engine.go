package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/driftguard/driftguard/internal/compliance"
	"github.com/driftguard/driftguard/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Engine detects drift between live and declared state
type Engine struct {
	db         *pgxpool.Pool
	compliance *compliance.Checker
	logger     *zap.Logger
}

func New(db *pgxpool.Pool, checker *compliance.Checker, logger *zap.Logger) *Engine {
	return &Engine{db: db, compliance: checker, logger: logger}
}

// UpsertResource saves a live resource snapshot (preserving any declared_state
// already recorded for it) and then runs drift + compliance detection.
func (e *Engine) UpsertResource(ctx context.Context, r *models.Resource) error {
	err := e.db.QueryRow(ctx, `
		INSERT INTO resources (org_id, provider, region, resource_type, resource_id, resource_name, live_state, last_scanned_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (org_id, provider, region, resource_type, resource_id)
		DO UPDATE SET
			resource_name   = EXCLUDED.resource_name,
			live_state      = EXCLUDED.live_state,
			last_scanned_at = EXCLUDED.last_scanned_at
		RETURNING id, declared_state
	`, r.OrgID, r.Provider, r.Region, r.ResourceType, r.ResourceID, r.ResourceName, r.LiveState).Scan(&r.ID, &r.DeclaredState)
	if err != nil {
		return fmt.Errorf("upserting resource: %w", err)
	}
	return e.DetectDrift(ctx, r)
}

// DetectDrift compares live state against declared state for a resource,
// creates/updates drift records, and evaluates compliance.
func (e *Engine) DetectDrift(ctx context.Context, r *models.Resource) error {
	// Compliance is evaluated on every scan, independent of drift.
	if err := e.evaluateCompliance(ctx, r); err != nil {
		e.logger.Warn("compliance evaluation failed", zap.String("resource_id", r.ResourceID), zap.Error(err))
	}

	// If no declared state, mark as unmanaged drift
	if r.DeclaredState == nil || len(r.DeclaredState) == 0 || string(r.DeclaredState) == "null" {
		return e.upsertDrift(ctx, r, "unmanaged", models.DriftDiff{}, nil, "low")
	}

	// Parse both states
	var live, declared map[string]interface{}
	if err := json.Unmarshal(r.LiveState, &live); err != nil {
		return fmt.Errorf("parsing live state: %w", err)
	}
	if err := json.Unmarshal(r.DeclaredState, &declared); err != nil {
		return fmt.Errorf("parsing declared state: %w", err)
	}

	// Compare fields
	diff := e.diffStates(declared, live)

	if len(diff.Fields) == 0 {
		// No drift — resolve any existing open drift records
		return e.resolveDrift(ctx, r.ID)
	}

	// Score severity based on what drifted
	severity := e.scoreSeverity(r.ResourceType, diff)

	return e.upsertDrift(ctx, r, "modified", diff, nil, severity)
}

// diffStates computes field-level differences between declared and live state
func (e *Engine) diffStates(declared, live map[string]interface{}) models.DriftDiff {
	var diff models.DriftDiff

	// Fields to ignore (internal/metadata fields)
	ignore := map[string]bool{
		"scanned_at": true,
		"created_at": true,
		"updated_at": true,
	}

	for key, declaredVal := range declared {
		if ignore[key] {
			continue
		}
		liveVal, exists := live[key]
		if !exists {
			diff.Fields = append(diff.Fields, models.DriftField{
				Field:    key,
				Declared: declaredVal,
				Live:     nil,
			})
			continue
		}
		if !reflect.DeepEqual(normalise(declaredVal), normalise(liveVal)) {
			diff.Fields = append(diff.Fields, models.DriftField{
				Field:    key,
				Declared: declaredVal,
				Live:     liveVal,
			})
		}
	}

	// Check for fields present in live but not in declared
	for key, liveVal := range live {
		if ignore[key] {
			continue
		}
		if _, exists := declared[key]; !exists {
			diff.Fields = append(diff.Fields, models.DriftField{
				Field:    key,
				Declared: nil,
				Live:     liveVal,
			})
		}
	}

	return diff
}

// scoreSeverity assigns a severity based on resource type and drifted fields
func (e *Engine) scoreSeverity(resourceType string, diff models.DriftDiff) string {
	criticalFields := map[string]map[string]bool{
		"ec2_instance": {
			"security_groups":        true,
			"iam_profile":            true,
			"termination_protection": true,
		},
		"s3_bucket": {
			"public_access_blocked": true,
			"encryption_enabled":    true,
		},
		"iam_role": {
			"assume_role_policy": true,
		},
	}

	highFields := map[string]map[string]bool{
		"ec2_instance": {
			"instance_type": true,
			"subnet_id":     true,
			"vpc_id":        true,
		},
		"s3_bucket": {
			"versioning_status": true,
		},
	}

	maxSeverity := "low"

	resourceCritical := criticalFields[resourceType]
	resourceHigh := highFields[resourceType]

	for _, field := range diff.Fields {
		if resourceCritical != nil && resourceCritical[field.Field] {
			return "critical" // can't get worse, return early
		}
		if resourceHigh != nil && resourceHigh[field.Field] {
			maxSeverity = "high"
		} else if maxSeverity == "low" {
			maxSeverity = "medium"
		}
	}

	return maxSeverity
}

// upsertDrift creates or updates a drift record
func (e *Engine) upsertDrift(ctx context.Context, r *models.Resource, driftType string, diff models.DriftDiff, changedBy *string, severity string) error {
	diffBytes, err := json.Marshal(diff)
	if err != nil {
		return fmt.Errorf("marshalling diff: %w", err)
	}

	changedByStr := ""
	if changedBy != nil {
		changedByStr = *changedBy
	}

	// One open drift per resource (enforced by uq_drift_open_per_resource).
	// Insert, or update the existing open record in place.
	var driftID uuid.UUID
	var inserted bool
	err = e.db.QueryRow(ctx, `
		INSERT INTO drift_records (org_id, resource_id, drift_type, severity, diff, changed_by, is_resolved)
		VALUES ($1, $2, $3, $4, $5, $6, false)
		ON CONFLICT (resource_id) WHERE is_resolved = false
		DO UPDATE SET
			drift_type = EXCLUDED.drift_type,
			severity   = EXCLUDED.severity,
			diff       = EXCLUDED.diff,
			changed_by = EXCLUDED.changed_by
		RETURNING id, (xmax = 0) AS inserted
	`, r.OrgID, r.ID, driftType, severity, diffBytes, changedByStr).Scan(&driftID, &inserted)
	if err != nil {
		return fmt.Errorf("upserting drift record: %w", err)
	}

	if inserted {
		e.logger.Info("new drift detected",
			zap.String("resource_type", r.ResourceType),
			zap.String("resource_id", r.ResourceID),
			zap.String("drift_type", driftType),
			zap.String("severity", severity),
			zap.Int("changed_fields", len(diff.Fields)),
		)
	}
	return nil
}

// evaluateCompliance runs built-in policy checks against a resource's live state
// and reconciles the compliance_violations table (delete-then-insert per resource).
func (e *Engine) evaluateCompliance(ctx context.Context, r *models.Resource) error {
	if e.compliance == nil {
		return nil
	}
	violations := e.compliance.Evaluate(r.ResourceType, r.LiveState)

	tx, err := e.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM compliance_violations WHERE resource_id = $1`, r.ID); err != nil {
		return err
	}
	for _, v := range violations {
		if _, err := tx.Exec(ctx, `
			INSERT INTO compliance_violations
				(org_id, resource_id, drift_id, policy_id, policy_name, framework, description, severity)
			VALUES ($1, $2, NULL, $3, $4, $5, $6, $7)
		`, r.OrgID, r.ID, v.PolicyID, v.PolicyName, v.Framework, v.Description, v.Severity); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// resolveDrift marks open drift records as resolved for a resource
func (e *Engine) resolveDrift(ctx context.Context, resourceID uuid.UUID) error {
	now := time.Now()
	_, err := e.db.Exec(ctx, `
		UPDATE drift_records
		SET is_resolved = true, resolved_at = $1
		WHERE resource_id = $2 AND is_resolved = false
	`, now, resourceID)
	return err
}

// GetDriftSummary returns dashboard counts
func (e *Engine) GetDriftSummary(ctx context.Context, orgID uuid.UUID) (*models.DriftSummary, error) {
	var summary models.DriftSummary

	err := e.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE NOT is_resolved)                          AS total_drifts,
			COUNT(*) FILTER (WHERE NOT is_resolved AND severity = 'critical') AS critical_drifts,
			COUNT(*) FILTER (WHERE NOT is_resolved AND severity = 'high')     AS high_drifts,
			COUNT(*) FILTER (WHERE NOT is_resolved AND severity = 'medium')   AS medium_drifts,
			COUNT(*) FILTER (WHERE NOT is_resolved AND severity = 'low')      AS low_drifts,
			COUNT(*) FILTER (WHERE NOT is_resolved)                          AS unresolved_count,
			COUNT(*) FILTER (WHERE is_resolved AND resolved_at > NOW() - INTERVAL '24h') AS resolved_today,
			COUNT(DISTINCT resource_id) FILTER (WHERE NOT is_resolved)       AS affected_resources
		FROM drift_records
		WHERE org_id = $1
	`, orgID).Scan(
		&summary.TotalDrifts,
		&summary.CriticalDrifts,
		&summary.HighDrifts,
		&summary.MediumDrifts,
		&summary.LowDrifts,
		&summary.UnresolvedCount,
		&summary.ResolvedToday,
		&summary.AffectedResources,
	)
	if err != nil {
		return &summary, err
	}

	// Resource-level compliance posture.
	err = e.db.QueryRow(ctx, `
		SELECT
			COUNT(*)                                                                  AS total_resources,
			COUNT(*) FILTER (WHERE v.cnt > 0)                                          AS noncompliant,
			COUNT(*) FILTER (WHERE COALESCE(v.cnt, 0) = 0)                             AS compliant
		FROM resources r
		LEFT JOIN (
			SELECT resource_id, COUNT(*) AS cnt FROM compliance_violations
			WHERE resource_id IS NOT NULL GROUP BY resource_id
		) v ON v.resource_id = r.id
		WHERE r.org_id = $1
	`, orgID).Scan(
		&summary.TotalResources,
		&summary.NoncompliantResources,
		&summary.CompliantResources,
	)
	return &summary, err
}

// normalise converts JSON numbers to float64 for consistent comparison
func normalise(v interface{}) interface{} {
	switch val := v.(type) {
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		return v
	}
}
