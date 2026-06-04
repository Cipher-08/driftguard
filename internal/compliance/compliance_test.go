package compliance

import (
	"encoding/json"
	"testing"
)

func state(t *testing.T, m map[string]interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func hasPolicy(vs []Violation, id string) bool {
	for _, v := range vs {
		if v.PolicyID == id {
			return true
		}
	}
	return false
}

func TestPublicS3BucketFlagged(t *testing.T) {
	c := New()
	vs := c.Evaluate("s3_bucket", state(t, map[string]interface{}{
		"public_access_blocked": false,
		"encryption_enabled":    true,
		"versioning_status":     "Enabled",
	}))
	if !hasPolicy(vs, "CIS-S3-1") {
		t.Fatalf("expected CIS-S3-1 (public access) violation, got %+v", vs)
	}
}

func TestCompliantS3BucketClean(t *testing.T) {
	c := New()
	vs := c.Evaluate("s3_bucket", state(t, map[string]interface{}{
		"public_access_blocked": true,
		"encryption_enabled":    true,
		"versioning_status":     "Enabled",
	}))
	if len(vs) != 0 {
		t.Fatalf("expected no violations, got %+v", vs)
	}
}

func TestUnencryptedBucketHighSeverity(t *testing.T) {
	c := New()
	vs := c.Evaluate("s3_bucket", state(t, map[string]interface{}{
		"public_access_blocked": true,
		"encryption_enabled":    false,
		"versioning_status":     "Enabled",
	}))
	if !hasPolicy(vs, "CIS-S3-2") {
		t.Fatalf("expected encryption violation, got %+v", vs)
	}
	for _, v := range vs {
		if v.PolicyID == "CIS-S3-2" && v.Severity != "high" {
			t.Fatalf("expected high severity, got %s", v.Severity)
		}
	}
}

func TestPublicGCSBucketFlagged(t *testing.T) {
	c := New()
	vs := c.Evaluate("gcs_bucket", state(t, map[string]interface{}{
		"uniform_bucket_level_access": true,
		"public":                      true,
	}))
	if !hasPolicy(vs, "CIS-GCP-STO-2") {
		t.Fatalf("expected public bucket violation, got %+v", vs)
	}
}

func TestUnknownResourceTypeNoViolations(t *testing.T) {
	c := New()
	if vs := c.Evaluate("unknown_type", state(t, map[string]interface{}{"x": 1})); len(vs) != 0 {
		t.Fatalf("expected no violations for unknown type, got %+v", vs)
	}
}
