// Package llm provides a pluggable, provider-agnostic interface for generating
// Infrastructure-as-Code remediation patches. It supports free providers
// (Groq, Google Gemini, local Ollama) so DriftGuard never requires a paid key.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// PatchRequest describes a single drift that needs a Terraform remediation.
type PatchRequest struct {
	ResourceType  string
	ResourceID    string
	Severity      string
	LiveState     json.RawMessage // actual cloud state
	DeclaredState json.RawMessage // desired state from IaC (may be empty/unmanaged)
	Diff          json.RawMessage // field-level diff produced by the engine
}

// Client generates remediation patches.
type Client interface {
	// GeneratePatch returns a Terraform HCL snippet that reconciles the drift.
	GeneratePatch(ctx context.Context, req PatchRequest) (string, error)
	// Name returns the provider identifier (e.g. "groq", "gemini", "ollama").
	Name() string
}

const systemPrompt = `You are a senior infrastructure engineer specialising in Terraform.
You are given the DESIRED (declared) state and the ACTUAL (live) state of a single cloud resource, plus a field-level diff.
Produce a minimal, correct Terraform HCL configuration that brings the live resource back in line with the desired state.
Rules:
- Output ONLY valid Terraform HCL. No markdown fences, no prose, no explanation.
- If the declared state is empty (the resource is unmanaged), generate a best-effort Terraform resource block that imports/describes the live resource so it can be brought under management.
- Prefer the smallest change that fixes the drift.`

// buildUserPrompt renders the drift into a single instruction string.
func buildUserPrompt(req PatchRequest) string {
	declared := string(req.DeclaredState)
	if strings.TrimSpace(declared) == "" || declared == "null" {
		declared = "(none — resource is unmanaged by IaC)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Resource type: %s\n", req.ResourceType)
	fmt.Fprintf(&b, "Resource id: %s\n", req.ResourceID)
	fmt.Fprintf(&b, "Drift severity: %s\n\n", req.Severity)
	fmt.Fprintf(&b, "DESIRED (declared) state:\n%s\n\n", declared)
	fmt.Fprintf(&b, "ACTUAL (live) state:\n%s\n\n", string(req.LiveState))
	if len(req.Diff) > 0 {
		fmt.Fprintf(&b, "Field-level diff:\n%s\n\n", string(req.Diff))
	}
	b.WriteString("Return the corrected Terraform HCL now.")
	return b.String()
}

// stripFences removes ```hcl / ``` wrappers some models add despite instructions.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
