// Package docs bundles the published guides with the CLI for offline reading.
package docs

import (
	"embed"
	"encoding/json"
	"path"
	"strings"
)

// Files contains the same Markdown sources and mapping used by docs-gen.
//
//go:embed index.md guides wiki.json
var Files embed.FS

type Guide struct {
	Source string `json:"source"`
	Title  string `json:"title"`
}

type Manifest struct {
	PageURL string  `json:"page_url"`
	Guides  []Guide `json:"guides"`
}

func Load() (Manifest, error) {
	data, err := Files.ReadFile("wiki.json")
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	err = json.Unmarshal(data, &manifest)
	return manifest, err
}

// Topic maps the documentation folders onto the help command hierarchy.
func (g Guide) Topic() []string {
	source := strings.TrimPrefix(g.Source, "guides/")
	source = strings.TrimSuffix(source, ".md")
	if path.Base(source) == "index" {
		source = strings.TrimSuffix(strings.TrimSuffix(source, "index"), "/")
	}
	if source == "" {
		return nil
	}
	return strings.Split(source, "/")
}
