// Package docs bundles the website's guides with the CLI for offline reading.
package docs

import (
	"embed"
	"encoding/json"
	"path"
	"strings"
)

// Files contains the same Markdown sources and manifest used by the website.
//
//go:embed index.md guides guides.json
var Files embed.FS

type Guide struct {
	Source    string `json:"source"`
	LegacyURL string `json:"legacy_url,omitempty"`
}

type Manifest struct {
	SiteURL string  `json:"site_url"`
	Guides  []Guide `json:"guides"`
}

func Load() (Manifest, error) {
	data, err := Files.ReadFile("guides.json")
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
