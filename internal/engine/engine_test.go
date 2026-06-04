package engine

import "testing"

func TestDiffStatesDetectsChangedField(t *testing.T) {
	e := &Engine{}
	declared := map[string]interface{}{"instance_type": "t3.micro", "monitoring": "enabled"}
	live := map[string]interface{}{"instance_type": "t3.large", "monitoring": "enabled"}

	diff := e.diffStates(declared, live)
	if len(diff.Fields) != 1 {
		t.Fatalf("expected 1 changed field, got %d: %+v", len(diff.Fields), diff.Fields)
	}
	if diff.Fields[0].Field != "instance_type" {
		t.Fatalf("expected instance_type to drift, got %s", diff.Fields[0].Field)
	}
}

func TestDiffStatesIgnoresMetadata(t *testing.T) {
	e := &Engine{}
	declared := map[string]interface{}{"name": "web", "scanned_at": "2020"}
	live := map[string]interface{}{"name": "web", "scanned_at": "2026"}

	if diff := e.diffStates(declared, live); len(diff.Fields) != 0 {
		t.Fatalf("expected metadata fields ignored, got %+v", diff.Fields)
	}
}

func TestDiffStatesDetectsLiveOnlyField(t *testing.T) {
	e := &Engine{}
	declared := map[string]interface{}{"name": "web"}
	live := map[string]interface{}{"name": "web", "public_ip": "1.2.3.4"}

	diff := e.diffStates(declared, live)
	if len(diff.Fields) != 1 || diff.Fields[0].Field != "public_ip" {
		t.Fatalf("expected public_ip to be flagged, got %+v", diff.Fields)
	}
}

func TestScoreSeverityCriticalField(t *testing.T) {
	e := &Engine{}
	declared := map[string]interface{}{"public_access_blocked": true}
	live := map[string]interface{}{"public_access_blocked": false}
	diff := e.diffStates(declared, live)

	if sev := e.scoreSeverity("s3_bucket", diff); sev != "critical" {
		t.Fatalf("expected critical severity for public access change, got %s", sev)
	}
}

func TestScoreSeverityLowForUnknownField(t *testing.T) {
	e := &Engine{}
	declared := map[string]interface{}{"description": "old"}
	live := map[string]interface{}{"description": "new"}
	diff := e.diffStates(declared, live)

	if sev := e.scoreSeverity("iam_role", diff); sev != "medium" {
		t.Fatalf("expected medium severity for generic field change, got %s", sev)
	}
}
