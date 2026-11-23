package devremote

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCodespaceCreateDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, repo, size, recipe, wantRepo, wantSize string
	}{
		{"one wiki", "", "", "mediawiki", "github/codespaces-blank", "basicLinux32gb"},
		{"wiki farm", "", "", farmRecipe, "github/codespaces-blank", "premiumLinux"},
		{"override size", "", "standardLinux32gb", farmRecipe, "github/codespaces-blank", "standardLinux32gb"},
		{"override repo", "example/workspace", "", "mediawiki", "example/workspace", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := codespaceCreateArgs("wiki", tc.repo, tc.size, tc.recipe, 15*time.Minute)
			for _, arg := range []string{"--repo=" + tc.wantRepo, "--display-name=wiki", "--idle-timeout=15m0s"} {
				if !slices.Contains(args, arg) {
					t.Errorf("missing %s in %v", arg, args)
				}
			}
			machineIndex := slices.IndexFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "--machine=") })
			if tc.wantSize == "" {
				if machineIndex >= 0 {
					t.Errorf("unexpected machine override: %s", args[machineIndex])
				}
			} else if machineIndex < 0 || args[machineIndex] != "--machine="+tc.wantSize {
				t.Errorf("expected machine type %s in %v", tc.wantSize, args)
			}
		})
	}
}
