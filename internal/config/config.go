/*Package config for interacting with the cli config

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package config

/*DevModeValues allowed values for DevMode.*/
var DevModeValues = AllowedOptions([]string{DevModeMwdd})

/*DevModeMwdd value for DevMode that will use the docker/mediawiki-docker-dev command set.*/
const DevModeMwdd string = "docker"

/*Config representation of a cli config.*/
type Config struct {
	DevMode string `json:"dev_mode"`
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
