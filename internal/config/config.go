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

	Gerrit struct {
		Username string `koanf:"username" json:"username"`
		Password string `koanf:"password" json:"password"`
		// InteractionType for git interaction with Gerrit.
		// Acceptable values are `http` and `ssh`.
		InteractionType string `koanf:"interaction_type" json:"interaction_type"`
	} `koanf:"gerrit" json:"gerrit"`
}

func defaultConfig() Config {
	return Config{
		TimerLastEmittedEvent:  timers.String(timers.NowUTC()),
		TimerLastUpdateChecked: timers.String(timers.NowUTC()),
	}
}
