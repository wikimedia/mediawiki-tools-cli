package ziki

import (
	"errors"
	"strings"
)

type Location struct {
	Description string
	Transitions []LocationName
	Events      []string
}

func (loc *Location) CanGoTo(locationName LocationName) bool {
	for _, name := range loc.Transitions {
		if name == locationName {
			return true
		}
	}
	return false
}

func LocationNameFromString(inputName string) (LocationName, error) {
	for _, locationName := range AllLocationNames {
		lowCase := strings.ToLower(string(locationName))
		short := lowCase[0:3]
		if (lowCase == inputName) || (short == inputName[0:3]) {
			return locationName, nil
		}
	}
	return "", errors.New("Can't find location " + inputName)
}
