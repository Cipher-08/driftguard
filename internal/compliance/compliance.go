// Package compliance evaluates a resource's live state against a set of
// built-in security/compliance rules (CIS / SOC2-flavoured). It is intentionally
// dependency-free so it can be extended without an external policy engine.
package compliance

import "encoding/json"

// Violation is a single failed policy check.
type Violation struct {
	PolicyID    string `json:"policy_id"`
	PolicyName  string `json:"policy_name"`
	Framework   string `json:"framework"` // cis, soc2, custom
	Description string `json:"description"`
	Severity    string `json:"severity"` // critical, high, medium, low
}

// Rule inspects a resource's decoded live state and returns a violation if it fails.
type Rule struct {
	ResourceType string
	Check        func(state map[string]interface{}) *Violation
}

// Checker holds the active rule set.
type Checker struct {
	rules []Rule
}

// New returns a Checker pre-loaded with the default rule set.
func New() *Checker {
	return &Checker{rules: defaultRules()}
}

// Evaluate decodes live state and runs every rule for the resource type.
func (c *Checker) Evaluate(resourceType string, liveState json.RawMessage) []Violation {
	var state map[string]interface{}
	if err := json.Unmarshal(liveState, &state); err != nil {
		return nil
	}
	var out []Violation
	for _, r := range c.rules {
		if r.ResourceType != resourceType {
			continue
		}
		if v := r.Check(state); v != nil {
			out = append(out, *v)
		}
	}
	return out
}

func asBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func defaultRules() []Rule {
	return []Rule{
		// ---- S3 ----
		{
			ResourceType: "s3_bucket",
			Check: func(s map[string]interface{}) *Violation {
				if !asBool(s["public_access_blocked"]) {
					return &Violation{
						PolicyID: "CIS-S3-1", PolicyName: "S3 Block Public Access enabled",
						Framework: "cis", Severity: "critical",
						Description: "Bucket does not have all four Block Public Access settings enabled, risking public exposure.",
					}
				}
				return nil
			},
		},
		{
			ResourceType: "s3_bucket",
			Check: func(s map[string]interface{}) *Violation {
				if !asBool(s["encryption_enabled"]) {
					return &Violation{
						PolicyID: "CIS-S3-2", PolicyName: "S3 default encryption enabled",
						Framework: "cis", Severity: "high",
						Description: "Bucket does not have server-side encryption enabled.",
					}
				}
				return nil
			},
		},
		{
			ResourceType: "s3_bucket",
			Check: func(s map[string]interface{}) *Violation {
				if asString(s["versioning_status"]) != "Enabled" {
					return &Violation{
						PolicyID: "SOC2-S3-1", PolicyName: "S3 versioning enabled",
						Framework: "soc2", Severity: "medium",
						Description: "Bucket versioning is not enabled, reducing recoverability from accidental deletion.",
					}
				}
				return nil
			},
		},
		// ---- EC2 ----
		{
			ResourceType: "ec2_instance",
			Check: func(s map[string]interface{}) *Violation {
				if ip := asString(s["public_ip"]); ip != "" {
					return &Violation{
						PolicyID: "CIS-EC2-1", PolicyName: "EC2 instance has public IP",
						Framework: "cis", Severity: "medium",
						Description: "Instance is assigned a public IP address, increasing its attack surface.",
					}
				}
				return nil
			},
		},
		{
			ResourceType: "ec2_instance",
			Check: func(s map[string]interface{}) *Violation {
				if asString(s["monitoring"]) == "disabled" {
					return &Violation{
						PolicyID: "SOC2-EC2-1", PolicyName: "EC2 detailed monitoring enabled",
						Framework: "soc2", Severity: "low",
						Description: "Detailed CloudWatch monitoring is disabled on the instance.",
					}
				}
				return nil
			},
		},
		// ---- GCP storage bucket ----
		{
			ResourceType: "gcs_bucket",
			Check: func(s map[string]interface{}) *Violation {
				if !asBool(s["uniform_bucket_level_access"]) {
					return &Violation{
						PolicyID: "CIS-GCP-STO-1", PolicyName: "Uniform bucket-level access enabled",
						Framework: "cis", Severity: "high",
						Description: "Bucket does not enforce uniform bucket-level access, allowing legacy ACLs.",
					}
				}
				return nil
			},
		},
		{
			ResourceType: "gcs_bucket",
			Check: func(s map[string]interface{}) *Violation {
				if asBool(s["public"]) {
					return &Violation{
						PolicyID: "CIS-GCP-STO-2", PolicyName: "Storage bucket not public",
						Framework: "cis", Severity: "critical",
						Description: "Bucket is publicly accessible (allUsers / allAuthenticatedUsers).",
					}
				}
				return nil
			},
		},
	}
}
