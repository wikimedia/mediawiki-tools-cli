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
	lowCaseInput := strings.ToLower(inputName)

	// Look for an exact match first
	for _, locationName := range AllLocationNames {
		lowCaseLoc := strings.ToLower(string(locationName))
		if lowCaseLoc == lowCaseInput {
			return locationName, nil
		}
	}

	// Fallback to look by letter for find a unique match
	for x := 1; x <= len(lowCaseInput) && x >= 0; x++ {
		shortMatches := []LocationName{}
		for _, locationName := range AllLocationNames {
			lowCaseLoc := strings.ToLower(string(locationName))
			if len(lowCaseLoc) >= x && lowCaseInput[0:x] == lowCaseLoc[0:x] {
				shortMatches = append(shortMatches, locationName)
			}
		}
		if len(shortMatches) == 1 {
			return shortMatches[0], nil
		}
	}

	return "", errors.New("Can't find location " + inputName)
}
