package config

import "gitlab.wikimedia.org/repos/releng/cli/internal/util/timers"

/*Config representation of a cli config.*/
type Config struct {
	// DevMode the style of dev environment that the `dev` command uses.
	// This is no longer really used, as the `dev` command is always an alias to `docker` now.
	DevMode string `koanf:"dev_mode" json:"dev_mode"`
	// Telemetry whether or not to send telemetry data.
	Telemetry string `koanf:"telemetry" json:"telemetry"`

	TimerLastEmittedEvent  string `koanf:"timer_last_emitted_event" json:"timer_last_emitted_event"`
	TimerLastUpdateChecked string `koanf:"timer_last_update_checked" json:"timer_last_update_checked"`

	Gerrit    GerritConfig    `koanf:"gerrit" json:"gerrit"`
	CloudVPS  CloudVPSConfig  `koanf:"cloud_vps" json:"cloud_vps"`
	MwDev     MwDevConfig     `koanf:"mw_dev" json:"mw_dev"`
	Developer DeveloperConfig `koanf:"developer" json:"developer"`
}

type DeveloperConfig struct {
	Username   string `koanf:"username" json:"username"`
	SSHKeyPath string `koanf:"ssh_key_path" json:"ssh_key_path"`
}

type GerritConfig struct {
	// Gerrit username / shell name.
	// Can be retrieved from the Username field of https://gerrit.wikimedia.org/r/settings
	Username string `koanf:"username" json:"username"`
	// Gerrit HTTP credentials.
	// Can be retrieved from https://gerrit.wikimedia.org/r/settings/#HTTPCredentials
	Password string `koanf:"password" json:"password"`
	// InteractionType for git interaction with Gerrit.
	// Acceptable values are `http` and `ssh`.
	InteractionType string `koanf:"interaction_type" json:"interaction_type"`
}

type CloudVPSConfig struct {
	DefaultProject string                     `koanf:"default_project" json:"default_project"`
	Projects       map[string]CloudVPSProject `koanf:"projects" json:"projects"`
}

type CloudVPSProject struct {
	Credential CloudVPSProjectCredential `koanf:"credential" json:"credential"`
}

type CloudVPSProjectCredential struct {
	ID     string `koanf:"id" json:"id"`
	Secret string `koanf:"secret" json:"secret"`
}

type MwDevConfig struct {
	Docker MwDevDockerConfig `koanf:"docker" json:"docker"`
}

type MwDevDockerConfig struct {
	// The default DB type to use for the mediawiki service at installation time.
	// One of sqlite, mysql, postgresql
	DBType string `koanf:"db_type" json:"db_type"`
}

func defaultConfig() Config {
	return Config{
		TimerLastEmittedEvent:  timers.String(timers.NowUTC()),
		TimerLastUpdateChecked: timers.String(timers.NowUTC()),
		Gerrit: GerritConfig{
			InteractionType: "ssh",
		},
		CloudVPS: CloudVPSConfig{
			Projects: make(map[string]CloudVPSProject),
		},
		Developer: DeveloperConfig{},
	}
}
