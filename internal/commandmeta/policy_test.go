package commandmeta

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAgentPolicy(t *testing.T) {
	for _, test := range []struct {
		name                    string
		hints                   Hints
		agent, preview, allowed bool
	}{
		{"human write", Hints{DataAccess: []DataAccess{DataUnknown}}, false, false, true},
		{"agent public read", Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true}, DataAccess: []DataAccess{DataPublic}}, true, false, true},
		{"agent restricted read needs resource checks", Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true}, DataAccess: []DataAccess{DataPrivateContent}}, true, false, true},
		{"agent account read", Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true}, DataAccess: []DataAccess{DataAccountMetadata}}, true, false, true},
		{"agent create", Hints{DataAccess: []DataAccess{DataSuppliedContent}}, true, false, false},
		{"agent edit", Hints{ToolAnnotations: ToolAnnotations{DestructiveHint: true}, DataAccess: []DataAccess{DataPrivateContent}}, true, false, false},
		{"agent idempotent write", Hints{ToolAnnotations: ToolAnnotations{IdempotentHint: true}, DataAccess: []DataAccess{DataAccountMetadata}}, true, false, false},
		{"agent preview", Hints{DataAccess: []DataAccess{DataSuppliedContent}}, true, true, true},
		{"agent secret read", Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true}, DataAccess: []DataAccess{DataSecrets}}, true, false, false},
		{"agent credential preview", Hints{SecretInput: true, DataAccess: []DataAccess{DataAccountMetadata}}, true, true, false},
		{"agent unknown preview", Hints{DataAccess: []DataAccess{DataUnknown}}, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "operation", Annotations: Annotations(test.hints)}
			err := CheckAgentPolicy(cmd, test.agent, test.preview)
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%t error=%v", test.allowed, err)
			}
			if err != nil && !strings.Contains(err.Error(), "human approval required") {
				t.Fatal(err)
			}
		})
	}
	if err := CheckAgentPolicy(&cobra.Command{Use: "missing"}, true, false); err == nil {
		t.Fatal("unclassified agent command allowed")
	}
}
