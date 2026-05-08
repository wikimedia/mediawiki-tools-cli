package phabricator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// PhabConfig holds the parsed configuration for a Phabricator site.
type PhabConfig struct {
	Key            string
	URL            string
	Username       string
	DefaultProject string
	CachePath      string
	SiteName       string
}

// loadConfig reads phab.cfg from standard locations and returns config for the given site.
// If site is empty, uses the [main] default value.
func loadConfig(site string) (*PhabConfig, error) {
	candidates := configSearchPaths()

	var cfg *ini.File
	var loadErr error
	for _, path := range candidates {
		cfg, loadErr = ini.Load(path)
		if loadErr == nil {
			break
		}
	}
	if loadErr != nil {
		return nil, fmt.Errorf("could not find phab.cfg; looked in: %s", strings.Join(candidates, ", "))
	}

	siteName := site
	if siteName == "" {
		siteName = cfg.Section("main").Key("default").String()
	}
	if siteName == "" {
		return nil, fmt.Errorf("no site specified and no [main] default in phab.cfg")
	}

	section := cfg.Section(siteName)
	config := &PhabConfig{
		SiteName:       siteName,
		Key:            section.Key("key").String(),
		URL:            strings.TrimRight(section.Key("url").MustString("https://phabricator.wikimedia.org"), "/"),
		Username:       section.Key("username").String(),
		DefaultProject: section.Key("default_project").String(),
		CachePath:      section.Key("cache_path").String(),
	}

	if config.Key == "" {
		return nil, fmt.Errorf("no API key for site %q in phab.cfg", siteName)
	}

	if config.CachePath == "" {
		cacheDir, _ := os.UserCacheDir()
		config.CachePath = filepath.Join(cacheDir, "phab", config.DefaultProject)
	} else if strings.HasPrefix(config.CachePath, "~/") {
		home, _ := os.UserHomeDir()
		config.CachePath = filepath.Join(home, config.CachePath[2:])
	}

	return config, nil
}

func configSearchPaths() []string {
	candidates := []string{"phab.cfg"}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "phab", "phab.cfg"))
	} else {
		home, _ := os.UserHomeDir()
		candidates = append(candidates, filepath.Join(home, ".config", "phab", "phab.cfg"))
	}
	home, _ := os.UserHomeDir()
	candidates = append(candidates, filepath.Join(home, ".phab", "phab.cfg"))
	candidates = append(candidates, "/etc/phab/phab.cfg")
	return candidates
}
