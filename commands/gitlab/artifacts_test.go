package gitlab

import (
	"bytes"
	"testing"
)

func TestArtifactPruneSummary(t *testing.T) {
	artifacts := []ciArtifact{{ID: "first", Size: 1 << 30}, {ID: "second", Size: 512 << 20}, {ID: "third", Size: 256 << 20}}
	for _, tc := range []struct {
		name  string
		ids   []string
		count int
		want  string
	}{
		{"all", []string{"first", "second", "third"}, 3, "Deleted 3 of 3 selected artifacts (1.8 GiB).\n"},
		{"partial", []string{"second"}, 1, "Deleted 1 of 3 selected artifacts (512.0 MiB).\n"},
		{"none", nil, 0, "Deleted 0 of 3 selected artifacts (0 B).\n"},
		{"duplicate response IDs", []string{"first", "first"}, 1, "Deleted 1 of 3 selected artifacts (1.0 GiB).\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := outputCIArtifactsPruneSummary(&out, artifacts, tc.count, tc.ids); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("summary = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestFormatArtifactSize(t *testing.T) {
	for _, tc := range []struct {
		size int64
		want string
	}{{0, "0 B"}, {999, "999 B"}, {1024, "1.0 KiB"}, {150 << 20, "150.0 MiB"}, {1 << 40, "1.0 TiB"}} {
		if got := formatArtifactSize(tc.size); got != tc.want {
			t.Errorf("formatArtifactSize(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}
