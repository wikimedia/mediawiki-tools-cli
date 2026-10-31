package phabricator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
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

// loadConfig reads only credentials saved in CLI profiles. External files must
// be explicitly imported and validated before commands can use their tokens.
func loadConfig(profile string) (*PhabConfig, error) {
	selected := strings.TrimSpace(profile)
	if selected == "" {
		selected = phabProfiles.Default()
	}
	if selected != "" && mwconfig.State().EffectiveKoanf.Bool("phabricator.profiles."+selected+".logged_out") {
		return nil, fmt.Errorf("profile %q is logged out; use phabricator auth login", selected)
	}
	cfg, err := configFromMWCLI(selected)
	if err == nil {
		return cfg, nil
	}
	if (selected != "" && mwconfig.HasCredential("phabricator.profiles."+selected+".key")) || mwconfig.HasCredential("phabricator.key") {
		return nil, err
	}
	if path := detectPhabConfig(); path != "" {
		return nil, fmt.Errorf("no saved Phabricator credentials; found %s. Import and validate it with: mw phab auth login --method phab-cfg --profile <name>", path)
	}
	return nil, fmt.Errorf("no saved Phabricator credentials; use mw phab auth login --profile <name>")
}

// Detection checks file metadata only; it never retrieves or tests a token.
func detectPhabConfig() string {
	for _, path := range configSearchPaths() {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

func importPhabConfig(path, section string) (*PhabConfig, error) {
	if path == "" {
		path = detectPhabConfig()
	}
	if path == "" {
		return nil, fmt.Errorf("no phab.cfg found; specify --config-file")
	}
	parsed, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("could not read phab.cfg at %s", path)
	}
	return configFromINI(parsed, section)
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

	endpoint, err := secrets.Endpoint(phabCfg.URL)
	if err != nil {
		return nil, err
	}
	phabCfg.URL = endpoint
	applyDefaultCachePath(phabCfg)
	return phabCfg, nil
}

func configFromMWCLI(site string) (*PhabConfig, error) {
	configState := mwconfig.State()

	siteName := strings.TrimSpace(site)
	if siteName == "" {
		siteName = phabProfiles.Default()
	}

	get := func(key string) string {
		if siteName != "" {
			siteKey := "phabricator.profiles." + siteName + "." + key
			if value := strings.TrimSpace(configState.EffectiveKoanf.String(siteKey)); value != "" {
				return value
			}
			if value := strings.TrimSpace(configState.OnDiskKoanf.String(siteKey)); value != "" {
				return value
			}
		}

		if siteName != "" {
			return ""
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
		URL:            strings.TrimRight(get("url"), "/"),
		Username:       get("username"),
		DefaultProject: get("default_project"),
		CachePath:      get("cache_path"),
	}

	if phabCfg.URL == "" {
		phabCfg.URL = "https://phabricator.wikimedia.org"
	}

	key := "phabricator.key"
	if siteName != "" {
		key = "phabricator.profiles." + siteName + ".key"
	}
	var err error
	phabCfg.Key, err = (mwconfig.Credential{Key: key, Service: "phabricator", Endpoint: phabCfg.URL, Account: phabCfg.Username}).Read()
	if err != nil {
		return nil, err
	}
	if phabCfg.Key == "" {
		return nil, fmt.Errorf("no Phabricator API key in CLI config")
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
