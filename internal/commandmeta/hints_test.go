package commandmeta

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHintsRoundTrip(t *testing.T) {
	for _, access := range []DataAccess{DataNone, DataPublic, DataPrivateContent, DataPrivateMetadata, DataAccountMetadata, DataSuppliedContent, DataSecrets, DataUnknown} {
		want := Hints{
			ToolAnnotations: ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
			DataAccess:      []DataAccess{access},
			SecretInput:     true,
		}
		cmd := &cobra.Command{Use: "read", Annotations: map[string]string{"other": "retained"}}
		if Set(cmd, want) != cmd {
			t.Fatal("Set must return the original command")
		}
		got, err := Read(cmd)
		if err != nil || !reflect.DeepEqual(got, want) || cmd.Annotations["other"] != "retained" {
			t.Fatalf("hints = %+v, error = %v, annotations = %v", got, err, cmd.Annotations)
		}
	}
	hints := ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	data, err := json.Marshal(hints)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}` {
		t.Fatalf("unexpected MCP annotation shape: %s", data)
	}
}

func TestHintsMissingInvalidAndNotInherited(t *testing.T) {
	valid := Hints{DataAccess: []DataAccess{DataUnknown}}
	parent := Set(&cobra.Command{Use: "parent"}, valid)
	child := &cobra.Command{Use: "child"}
	parent.AddCommand(child)
	if _, err := Read(child); !errors.Is(err, ErrMissingHints) {
		t.Fatalf("child must not inherit hints: %v", err)
	}
	for _, test := range []struct {
		name, key, value, want string
	}{
		{"missing boolean", ReadOnlyAnnotation, "", "must be a boolean"},
		{"invalid boolean", IdempotentAnnotation, "maybe", "must be a boolean"},
		{"missing secret input", SecretInputAnnotation, "", "must be a boolean"},
		{"missing exposure", DataAccessAnnotation, "", "invalid data category"},
		{"invalid exposure", DataAccessAnnotation, "confidential-ish", "invalid data category"},
		{"duplicate exposure", DataAccessAnnotation, "public,public", "duplicate category"},
		{"none with another category", DataAccessAnnotation, "none,public", "must be used alone"},
		{"unknown with another category", DataAccessAnnotation, "unknown,account-metadata", "must be used alone"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := Set(&cobra.Command{Use: "test"}, valid)
			cmd.Annotations[test.key] = test.value
			if _, err := Read(cmd); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
	cmd := Set(&cobra.Command{Use: "contradictory"}, Hints{ToolAnnotations: ToolAnnotations{ReadOnlyHint: true, DestructiveHint: true}, DataAccess: []DataAccess{DataNone}})
	if _, err := Read(cmd); err == nil {
		t.Fatal("contradictory hints accepted")
	}
}

func TestMultipleDataCategories(t *testing.T) {
	want := Hints{
		DataAccess: []DataAccess{DataSuppliedContent, DataPrivateMetadata},
	}
	cmd := &cobra.Command{Use: "create", Annotations: Annotations(want)}
	got, err := Read(cmd)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("hints = %+v, error = %v", got, err)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"dataAccess":["supplied-content","potentially-private-metadata"]`) ||
		!strings.Contains(string(data), `"secretInput":false`) {
		t.Fatalf("unexpected mw metadata: %s", data)
	}
}
