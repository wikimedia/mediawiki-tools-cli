package mediawiki

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
)

func TestBranchCompatibility(t *testing.T) {
	for _, tc := range []struct {
		core, other string
		want        bool
	}{
		{"REL1_45", "REL1_45", false},
		{"REL1_45", "REL1_44", true},
		{"master", "REL1_45", true},
		{"REL1_45", "master", true},
		{"master", "main", false},
		{"feature/my-work", "master", false},
		{"REL1_45", "detached@abc123", false},
	} {
		if got := branchesLikelyIncompatible(tc.core, tc.other); got != tc.want {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
}

func TestCheckoutBranches(t *testing.T) {
	dir := t.TempDir()
	repos := map[string]string{"": "REL1_45", "skins/Vector": "REL1_45", "extensions/Example": "master"}
	for sub, branch := range repos {
		path := filepath.Join(dir, sub)
		if output, err := osexec.Command("git", "init", path).CombinedOutput(); err != nil {
			t.Fatalf("%s %v", output, err)
		}
		if err := osexec.Command("git", "-C", path, "symbolic-ref", "HEAD", "refs/heads/"+branch).Run(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "extensions", "PlainFolder"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := MediaWiki(dir).CheckoutBranches()
	if err != nil || len(got) != 3 {
		t.Fatalf("%+v %v", got, err)
	}
	if got[0].Name != "core" || got[1].Warning != "" || got[2].Warning == "" {
		t.Fatalf("%+v", got)
	}
}
