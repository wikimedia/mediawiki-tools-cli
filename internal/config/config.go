package config

/*DevModeValues allowed values for DevMode.*/
var DevModeValues = AllowedOptions([]string{DevModeMwdd})

/*DevModeMwdd value for DevMode that will use the docker/mediawiki-docker-dev command set.*/
const DevModeMwdd string = "docker"

/*Config representation of a cli config.*/
type Config struct {
	DevMode                string `json:"dev_mode"`
	Telemetry              string `json:"telemetry"`
	TimerLastEmmitedEvent  string `json:"_timer_last_emitted_event"`
	TimerLastUpdateChecked string `json:"_timer_last_update_checked"`
}

/*AllowedOptions representation of allowed options for a config value.*/
type AllowedOptions []string

/*Contains do the allowed options contain this value.*/
func (cao AllowedOptions) Contains(value string) bool {
	for _, v := range cao {
		if v == value {
			return true
		}
	}

	return false
}
