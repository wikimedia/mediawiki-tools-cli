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
}

func defaultConfig() Config {
	return Config{
		TimerLastEmittedEvent:  timers.String(timers.NowUTC()),
		TimerLastUpdateChecked: timers.String(timers.NowUTC()),
	}
}
