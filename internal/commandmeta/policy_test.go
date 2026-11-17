package commandmeta

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAgentPolicyModes(t *testing.T) {
	for _, policy := range []AgentPolicy{PolicyPermissive, PolicyBalanced, PolicyCautious} {
		for _, test := range []struct {
			name  string
			hints Hints
			high  bool
		}{
			{"public read", Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true}, DataAccess: []DataAccess{DataPublic}}, false},
			{"restricted read still needs resource checks", Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true}, DataAccess: []DataAccess{DataPrivateContent}}, false},
			{"create", Hints{DataAccess: []DataAccess{DataSuppliedContent}}, false},
			{"edit", Hints{ToolAnnotations: ToolAnnotations{DestructiveHint: true}, DataAccess: []DataAccess{DataPrivateContent}}, false},
			{"idempotent write", Hints{ToolAnnotations: ToolAnnotations{IdempotentHint: true}, DataAccess: []DataAccess{DataAccountMetadata}}, false},
			{"explicit high risk", Hints{HighRisk: true, DataAccess: []DataAccess{DataAccountMetadata}}, true},
			{"secret read", Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true}, DataAccess: []DataAccess{DataSecrets}}, true},
			{"credential input", Hints{SecretInput: true, DataAccess: []DataAccess{DataAccountMetadata}}, true},
			{"unknown API", Hints{DataAccess: []DataAccess{DataUnknown}}, true},
		} {
			t.Run(string(policy)+"/"+test.name, func(t *testing.T) {
				cmd := &cobra.Command{Use: "operation", Annotations: Annotations(test.hints)}
				cmd.Flags().Bool("dry-run", false, "Preview")
				requireApproval := test.high || policy == PolicyCautious || (policy == PolicyBalanced && !test.hints.ReadOnlyHint)
				for _, approved := range []bool{false, true} {
					err := CheckAgentPolicy(cmd, policy, true, false, approved)
					if (err != nil) != (requireApproval && !approved) {
						t.Fatalf("approved=%t requireApproval=%t err=%v", approved, requireApproval, err)
					}
					if err != nil && (!strings.Contains(err.Error(), "ask the user") || !strings.Contains(err.Error(), "--yes") || !strings.Contains(err.Error(), "--dry-run")) {
						t.Fatal(err)
					}
				}
				if err := CheckAgentPolicy(cmd, policy, true, true, false); err != nil {
					t.Fatalf("verified offline preview blocked: %v", err)
				}
				if err := CheckAgentPolicy(cmd, policy, false, false, false); err != nil {
					t.Fatalf("human affected: %v", err)
				}
			})
		}
	}
	for _, approved := range []bool{false, true} {
		if err := CheckAgentPolicy(&cobra.Command{Use: "missing"}, PolicyBalanced, true, false, approved); err == nil {
			t.Fatal("unclassified agent command allowed")
		}
	}
	if _, err := ParseAgentPolicy("typo"); err == nil {
		t.Fatal("invalid policy accepted")
	}
}
