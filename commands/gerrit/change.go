package gerrit

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

const gerritRequestTimeout = 10 * time.Second

var gerritChangePath = regexp.MustCompile(`^/r/(?:c/(?:[^/]+/)+\+/)?([0-9]+)(?:/([0-9]+))?/?$`)

func parseGerritChange(selector string, patchset int) (change, selectedPatchset int, err error) {
	selectedPatchset = patchset
	if number, err := strconv.Atoi(selector); err == nil && number > 0 {
		change = number
		return change, selectedPatchset, nil
	}
	address, err := url.Parse(selector)
	if err != nil || address.Scheme != "https" || address.Host != "gerrit.wikimedia.org" || address.User != nil {
		return change, selectedPatchset, fmt.Errorf("expected a Wikimedia Gerrit change number or URL, got %q", selector)
	}
	match := gerritChangePath.FindStringSubmatch(address.Path)
	if match == nil {
		return change, selectedPatchset, fmt.Errorf("Gerrit URL does not identify a change: %q", selector)
	}
	change, err = strconv.Atoi(match[1])
	if err != nil || change < 1 {
		return change, selectedPatchset, fmt.Errorf("invalid Gerrit change number in %q", selector)
	}
	if match[2] != "" {
		number, err := strconv.Atoi(match[2])
		if err != nil || number < 1 {
			return change, selectedPatchset, fmt.Errorf("invalid patchset number in %q", selector)
		}
		if patchset != 0 && patchset != number {
			return change, selectedPatchset, fmt.Errorf("--patchset %d conflicts with patchset %d in the URL", patchset, number)
		}
		selectedPatchset = number
	}
	return change, selectedPatchset, nil
}
