package ziki

import (
	"errors"
	"strings"
)

type Location struct {
	Description string
	Transitions []string
	Events      []string
}

func (loc *Location) CanGoTo(locName string) bool {
	for _, name := range loc.Transitions {
		if (strings.ToLower(name) == locName) || (strings.ToLower(name[0:3]) == locName[0:3]) {
			return true
		}
	}
	return false
}

func FindLocationName(inputName string) (string, error) {
	for key := range LocationMap {
		if (strings.ToLower(key) == inputName) || (strings.ToLower(key[0:3]) == inputName[0:3]) {
			return key, nil
		}
	}
	return "", errors.New("Can't find location " + inputName)
}
