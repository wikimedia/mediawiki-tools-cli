package files

import (
	"testing"

	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

func TestSyncerIgnoreFiles_KeepCustomComposeFiles(t *testing.T) {
	ignore := syncer("/tmp").IgnoreFiles

	cases := []struct {
		name     string
		path     string
		expected bool
	}{
		{name: "custom yml", path: "custom.yml", expected: true},
		{name: "custom yaml", path: "custom.yaml", expected: true},
		{name: "custom dashed", path: "custom-two.yml", expected: true},
		{name: "custom dashed multi", path: "custom-two-extra.yaml", expected: true},
		{name: "custom dotted", path: "custom.local.yml", expected: true},
		{name: "custom mixed", path: "custom.local-dev_1.yaml", expected: true},
		{name: "non custom", path: "compose/base/compose.yml", expected: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringsutil.StringInRegexSlice(tc.path, ignore)
			if got != tc.expected {
				t.Fatalf("StringInRegexSlice(%q, IgnoreFiles) = %v, want %v", tc.path, got, tc.expected)
			}
		})
	}
}
