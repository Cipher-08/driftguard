package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Migrate runs all SQL migrations embedded in the binary
func Migrate(databaseURL string) error {
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		return fmt.Errorf("connecting for migration: %w", err)
	}
	defer conn.Close(context.Background())

	// Create migrations table
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	for _, m := range migrations {
		var exists bool
		err := conn.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
			m.version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking migration %s: %w", m.version, err)
		}

		if exists {
			continue
		}

		if _, err := conn.Exec(context.Background(), m.sql); err != nil {
			return fmt.Errorf("running migration %s: %w", m.version, err)
		}

		if _, err := conn.Exec(context.Background(),
			`INSERT INTO schema_migrations (version) VALUES ($1)`,
			m.version,
		); err != nil {
			return fmt.Errorf("recording migration %s: %w", m.version, err)
		}
	}

	return nil
}

type migration struct {
	version string
	sql     string
}

var migrations = []migration{
	{
		version: "001_initial",
		sql: `
-- Organizations (multi-tenant)
CREATE TABLE IF NOT EXISTS organizations (
	id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name        TEXT NOT NULL,
	slug        TEXT NOT NULL UNIQUE,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE IF NOT EXISTS users (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	email           TEXT NOT NULL UNIQUE,
	password_hash   TEXT NOT NULL,
	name            TEXT NOT NULL,
	role            TEXT NOT NULL DEFAULT 'member', -- admin, member
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cloud credentials per org
CREATE TABLE IF NOT EXISTS cloud_credentials (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	provider        TEXT NOT NULL, -- aws, gcp, azure
	name            TEXT NOT NULL,
	credentials     JSONB NOT NULL, -- encrypted credential fields
	is_active       BOOLEAN NOT NULL DEFAULT true,
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- IaC repos connected per org
CREATE TABLE IF NOT EXISTS iac_repos (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	provider        TEXT NOT NULL, -- github, gitlab
	repo_url        TEXT NOT NULL,
	branch          TEXT NOT NULL DEFAULT 'main',
	iac_type        TEXT NOT NULL DEFAULT 'terraform', -- terraform, pulumi, cloudformation
	root_path       TEXT NOT NULL DEFAULT '/',
	access_token    TEXT NOT NULL,
	is_active       BOOLEAN NOT NULL DEFAULT true,
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Resources: live state snapshots
CREATE TABLE IF NOT EXISTS resources (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	provider        TEXT NOT NULL,
	region          TEXT NOT NULL,
	resource_type   TEXT NOT NULL, -- ec2_instance, s3_bucket, iam_role, etc.
	resource_id     TEXT NOT NULL,
	resource_name   TEXT,
	live_state      JSONB NOT NULL,
	declared_state  JSONB,
	last_scanned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE(org_id, provider, region, resource_type, resource_id)
);

-- Drift records
CREATE TABLE IF NOT EXISTS drift_records (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	resource_id     UUID NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
	drift_type      TEXT NOT NULL, -- modified, unmanaged, deleted
	severity        TEXT NOT NULL, -- critical, high, medium, low
	diff            JSONB NOT NULL, -- field-level diff
	changed_by      TEXT,          -- from CloudTrail / git blame
	changed_at      TIMESTAMPTZ,
	is_resolved     BOOLEAN NOT NULL DEFAULT false,
	resolved_at     TIMESTAMPTZ,
	resolved_by     UUID REFERENCES users(id),
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Compliance violations per drift record
CREATE TABLE IF NOT EXISTS compliance_violations (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	drift_id        UUID NOT NULL REFERENCES drift_records(id) ON DELETE CASCADE,
	policy_id       TEXT NOT NULL,
	policy_name     TEXT NOT NULL,
	framework       TEXT NOT NULL, -- cis, soc2, custom
	description     TEXT NOT NULL,
	severity        TEXT NOT NULL,
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Remediation suggestions (AI-generated)
CREATE TABLE IF NOT EXISTS remediations (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	drift_id        UUID NOT NULL REFERENCES drift_records(id) ON DELETE CASCADE,
	patch           TEXT NOT NULL, -- terraform HCL patch
	pr_url          TEXT,
	pr_number       INT,
	status          TEXT NOT NULL DEFAULT 'pending', -- pending, pr_opened, applied, dismissed
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit log
CREATE TABLE IF NOT EXISTS audit_log (
	id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	user_id     UUID REFERENCES users(id),
	action      TEXT NOT NULL,
	entity_type TEXT,
	entity_id   UUID,
	metadata    JSONB,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_drift_records_org_id ON drift_records(org_id);
CREATE INDEX IF NOT EXISTS idx_drift_records_resource_id ON drift_records(resource_id);
CREATE INDEX IF NOT EXISTS idx_drift_records_is_resolved ON drift_records(is_resolved);
CREATE INDEX IF NOT EXISTS idx_drift_records_created_at ON drift_records(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_resources_org_id ON resources(org_id);
CREATE INDEX IF NOT EXISTS idx_resources_provider ON resources(provider);
CREATE INDEX IF NOT EXISTS idx_compliance_violations_drift_id ON compliance_violations(drift_id);
`,
	},
}
