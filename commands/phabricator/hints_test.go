package phabricator

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

func TestPhabricatorHintCoverage(t *testing.T) {
	root := NewPhabricatorCmd()
	count := 0
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		if cmd.Runnable() {
			count++
			if _, err := commandmeta.Read(cmd); err != nil {
				t.Errorf("%s: %v", cmd.CommandPath(), err)
			}
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(root)
	if count != 35 {
		t.Fatalf("classified %d executable Phabricator commands, want 35", count)
	}
}

func TestPhabricatorHintSemantics(t *testing.T) {
	root := NewPhabricatorCmd()
	for _, test := range []struct {
		path        string
		readOnly    bool
		destructive bool
		idempotent  bool
		openWorld   bool
		data        []commandmeta.DataAccess
		secretInput bool
	}{
		{"mcp", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task view", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"view", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"paste view", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"project list", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateMetadata}, false},
		{"board columns", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateMetadata}, false},
		{"browse", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task create", false, false, false, true, []commandmeta.DataAccess{commandmeta.DataSuppliedContent, commandmeta.DataPrivateMetadata}, false},
		{"paste create", false, false, false, true, []commandmeta.DataAccess{commandmeta.DataSuppliedContent}, false},
		{"task edit", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata, commandmeta.DataSuppliedContent}, false},
		{"paste edit", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataSuppliedContent}, false},
		{"shell", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"api", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataUnknown}, false},
		{"auth status", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataAccountMetadata}, false},
		{"auth login", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataAccountMetadata}, true},
		{"auth profiles", true, false, true, false, []commandmeta.DataAccess{commandmeta.DataAccountMetadata}, false},
		{"auth default", false, true, true, false, []commandmeta.DataAccess{commandmeta.DataAccountMetadata}, false},
		{"auth remove", false, true, false, false, []commandmeta.DataAccess{commandmeta.DataAccountMetadata}, false},
		{"comments", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"read", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task list", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task comments", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task read", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"project view", true, false, true, true, []commandmeta.DataAccess{commandmeta.DataPrivateMetadata}, false},
		{"create", false, false, false, true, []commandmeta.DataAccess{commandmeta.DataSuppliedContent, commandmeta.DataPrivateMetadata}, false},
		{"assign", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"mv", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"rmtag", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"setprio", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"setstatus", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task assign", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task move", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task rmtag", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task priority", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
		{"task status", false, true, false, true, []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata}, false},
	} {
		t.Run(test.path, func(t *testing.T) {
			cmd, _, err := root.Find(strings.Fields(test.path))
			if err != nil {
				t.Fatal(err)
			}

			got, err := commandmeta.Read(cmd)
			want := commandmeta.Hints{
				HighRisk: test.path == "api" || test.path == "shell" || test.path == "auth login" || test.path == "auth remove",
				ToolAnnotations: commandmeta.ToolAnnotations{
					ReadOnlyHint: test.readOnly, DestructiveHint: test.destructive,
					IdempotentHint: test.idempotent, OpenWorldHint: test.openWorld,
				},
				DataAccess: test.data, SecretInput: test.secretInput,
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("hints = %+v, error = %v, want %+v", got, err, want)
			}
		})
	}
	for _, pair := range [][2]string{{"assign", "task assign"}, {"mv", "task move"}, {"setprio", "task priority"}, {"setstatus", "task status"}, {"create", "task create"}, {"v", "view"}, {"task mv", "task move"}} {
		first, _, err := root.Find(strings.Fields(pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		second, _, err := root.Find(strings.Fields(pair[1]))
		if err != nil {
			t.Fatal(err)
		}
		a, errA := commandmeta.Read(first)
		b, errB := commandmeta.Read(second)
		if errA != nil || errB != nil || !reflect.DeepEqual(a, b) {
			t.Errorf("%s and %s differ: %+v / %+v (%v / %v)", pair[0], pair[1], a, b, errA, errB)
		}
	}
}

func TestPhabricatorHelpExplainsDataAndEffects(t *testing.T) {
	root := NewPhabricatorCmd()
	for _, test := range []struct {
		path string
		want []string
	}{
		{"auth status", []string{"user.whoami", "masked", "not task or paste content"}},
		{"task create", []string{"printed before creation", "Existing task content is not fetched", "public-only"}},
		{"paste create", []string{"preview prints", "may be public"}},
		{"paste edit", []string{"without fetching", "preview prints", "visibility, are left unchanged", "not covered"}},
		{"shell", []string{"update task", "immediately", "without a separate confirmation"}},
	} {
		t.Run(test.path, func(t *testing.T) {
			cmd, _, err := root.Find(strings.Fields(test.path))
			if err != nil {
				t.Fatal(err)
			}
			for _, text := range test.want {
				if !strings.Contains(cmd.Long, text) {
					t.Errorf("help for %s does not explain %q", test.path, text)
				}
			}
		})
	}
}
