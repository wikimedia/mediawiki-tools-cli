package recipe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRecipeExamples(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "mount", "dev", "recipes", "*.yml"))
	if err != nil {
		t.Fatalf("glob mount/dev/recipes/*.yml: %v", err)
	}

	if len(paths) == 0 {
		t.Fatalf("no recipe examples found at mount/dev/recipes/*.yml")
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}

			if _, parseErr := Parse(raw); parseErr != nil {
				t.Fatalf("parse %s: %v", path, parseErr)
			}
		})
	}
}
