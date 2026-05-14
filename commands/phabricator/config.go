package phabricator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
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

	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		cfg, err := ini.Load(path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}

		config, err := configFromINI(cfg, site)
		if err != nil {
			return nil, err
		}
		return config, nil
	}

	config, err := configFromMWCLI(site)
	if err == nil {
		return config, nil
	}

	return nil, fmt.Errorf(
		"could not find usable Phabricator config. looked for phab.cfg in: %s; and mwcli config at: %s",
		strings.Join(candidates, ", "),
		mwconfig.Path(),
	)
}

func configFromINI(cfg *ini.File, site string) (*PhabConfig, error) {
	siteName := site
	if siteName == "" {
		siteName = cfg.Section("main").Key("default").String()
	}
	if siteName == "" {
		return nil, fmt.Errorf("no site specified and no [main] default in phab.cfg")
	}

	section := cfg.Section(siteName)
	phabCfg := &PhabConfig{
		SiteName:       siteName,
		Key:            section.Key("key").String(),
		URL:            strings.TrimRight(section.Key("url").MustString("https://phabricator.wikimedia.org"), "/"),
		Username:       section.Key("username").String(),
		DefaultProject: section.Key("default_project").String(),
		CachePath:      section.Key("cache_path").String(),
	}

	if phabCfg.Key == "" {
		return nil, fmt.Errorf("no API key for site %q in phab.cfg", siteName)
	}

	applyDefaultCachePath(phabCfg)
	return phabCfg, nil
}

func configFromMWCLI(site string) (*PhabConfig, error) {
	configState := mwconfig.State()

	siteName := strings.TrimSpace(site)
	if siteName == "" {
		siteName = strings.TrimSpace(configState.EffectiveKoanf.String("phabricator.default_site"))
		if siteName == "" {
			siteName = strings.TrimSpace(configState.OnDiskKoanf.String("phabricator.default_site"))
		}
		if siteName == "" {
			siteName = strings.TrimSpace(configState.EffectiveKoanf.String("phabricator.site"))
		}
		if siteName == "" {
			siteName = strings.TrimSpace(configState.OnDiskKoanf.String("phabricator.site"))
		}
	}

	get := func(key string) string {
		if siteName != "" {
			siteKey := "phabricator.sites." + siteName + "." + key
			if value := strings.TrimSpace(configState.EffectiveKoanf.String(siteKey)); value != "" {
				return value
			}
			if value := strings.TrimSpace(configState.OnDiskKoanf.String(siteKey)); value != "" {
				return value
			}
		}

		flatKey := "phabricator." + key
		if value := strings.TrimSpace(configState.EffectiveKoanf.String(flatKey)); value != "" {
			return value
		}
		if value := strings.TrimSpace(configState.OnDiskKoanf.String(flatKey)); value != "" {
			return value
		}

		return ""
	}

	phabCfg := &PhabConfig{
		SiteName:       siteName,
		Key:            get("key"),
		URL:            strings.TrimRight(get("url"), "/"),
		Username:       get("username"),
		DefaultProject: get("default_project"),
		CachePath:      get("cache_path"),
	}

	if phabCfg.URL == "" {
		phabCfg.URL = "https://phabricator.wikimedia.org"
	}

	if phabCfg.Key == "" {
		return nil, fmt.Errorf("no Phabricator API key in mwcli config")
	}

	if phabCfg.SiteName == "" {
		phabCfg.SiteName = "default"
	}

	applyDefaultCachePath(phabCfg)
	return phabCfg, nil
}

func applyDefaultCachePath(config *PhabConfig) {
	if config.CachePath == "" {
		cacheDir, _ := os.UserCacheDir()
		config.CachePath = filepath.Join(cacheDir, "phab", config.DefaultProject)
	} else if strings.HasPrefix(config.CachePath, "~/") {
		home, _ := os.UserHomeDir()
		config.CachePath = filepath.Join(home, config.CachePath[2:])
	}
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
