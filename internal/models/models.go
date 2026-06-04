package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Organization is a tenant
type Organization struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// User belongs to an org
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	OrgID        uuid.UUID `json:"org_id" db:"org_id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Name         string    `json:"name" db:"name"`
	Role         string    `json:"role" db:"role"` // admin, member
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Resource represents a single cloud resource (live snapshot)
type Resource struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	OrgID         uuid.UUID       `json:"org_id" db:"org_id"`
	Provider      string          `json:"provider" db:"provider"` // aws, gcp, azure
	Region        string          `json:"region" db:"region"`
	ResourceType  string          `json:"resource_type" db:"resource_type"`
	ResourceID    string          `json:"resource_id" db:"resource_id"`
	ResourceName  string          `json:"resource_name" db:"resource_name"`
	LiveState     json.RawMessage `json:"live_state" db:"live_state"`
	DeclaredState json.RawMessage `json:"declared_state,omitempty" db:"declared_state"`
	LastScannedAt time.Time       `json:"last_scanned_at" db:"last_scanned_at"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

// DriftRecord captures a detected drift
type DriftRecord struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	OrgID      uuid.UUID       `json:"org_id" db:"org_id"`
	ResourceID uuid.UUID       `json:"resource_id" db:"resource_id"`
	DriftType  string          `json:"drift_type" db:"drift_type"` // modified, unmanaged, deleted
	Severity   string          `json:"severity" db:"severity"`     // critical, high, medium, low
	Diff       json.RawMessage `json:"diff" db:"diff"`
	ChangedBy  string          `json:"changed_by" db:"changed_by"`
	ChangedAt  *time.Time      `json:"changed_at" db:"changed_at"`
	IsResolved bool            `json:"is_resolved" db:"is_resolved"`
	ResolvedAt *time.Time      `json:"resolved_at" db:"resolved_at"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	// Joined fields
	Resource   *Resource             `json:"resource,omitempty"`
	Violations []ComplianceViolation `json:"violations,omitempty"`
}

// DriftField represents a single changed field
type DriftField struct {
	Field    string      `json:"field"`
	Declared interface{} `json:"declared"`
	Live     interface{} `json:"live"`
}

// DriftDiff is the structured diff stored in drift_records.diff
type DriftDiff struct {
	Fields []DriftField `json:"fields"`
}

// ComplianceViolation is a policy violation linked to a drift record
type ComplianceViolation struct {
	ID          uuid.UUID `json:"id" db:"id"`
	OrgID       uuid.UUID `json:"org_id" db:"org_id"`
	DriftID     uuid.UUID `json:"drift_id" db:"drift_id"`
	PolicyID    string    `json:"policy_id" db:"policy_id"`
	PolicyName  string    `json:"policy_name" db:"policy_name"`
	Framework   string    `json:"framework" db:"framework"` // cis, soc2, custom
	Description string    `json:"description" db:"description"`
	Severity    string    `json:"severity" db:"severity"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Remediation is an AI-generated fix
type Remediation struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrgID     uuid.UUID `json:"org_id" db:"org_id"`
	DriftID   uuid.UUID `json:"drift_id" db:"drift_id"`
	Patch     string    `json:"patch" db:"patch"`
	PRURL     string    `json:"pr_url" db:"pr_url"`
	PRNumber  int       `json:"pr_number" db:"pr_number"`
	Status    string    `json:"status" db:"status"` // pending, pr_opened, applied, dismissed
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// DriftSummary is the API response for dashboard overview
type DriftSummary struct {
	TotalDrifts       int `json:"total_drifts"`
	CriticalDrifts    int `json:"critical_drifts"`
	HighDrifts        int `json:"high_drifts"`
	MediumDrifts      int `json:"medium_drifts"`
	LowDrifts         int `json:"low_drifts"`
	UnresolvedCount   int `json:"unresolved_count"`
	ResolvedToday     int `json:"resolved_today"`
	AffectedResources int `json:"affected_resources"`
	// Compliance posture (resource-level)
	TotalResources        int `json:"total_resources"`
	CompliantResources    int `json:"compliant_resources"`
	NoncompliantResources int `json:"noncompliant_resources"`
}

// CloudCredential stores cloud provider credentials per org
type CloudCredential struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	OrgID       uuid.UUID       `json:"org_id" db:"org_id"`
	Provider    string          `json:"provider" db:"provider"`
	Name        string          `json:"name" db:"name"`
	Credentials json.RawMessage `json:"-" db:"credentials"`
	IsActive    bool            `json:"is_active" db:"is_active"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}
