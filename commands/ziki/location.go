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
	lowCaseInput := normalizeDestination(inputName)

	// Look for an exact match first
	for _, locationName := range AllLocationNames {
		lowCaseLoc := strings.ToLower(string(locationName))
		if lowCaseLoc == lowCaseInput {
			return locationName, nil
		}
	}

	for _, destination := range AllLocationNames {
		for _, alias := range locationAliases[destination] {
			if normalizeDestination(alias) == lowCaseInput {
				return destination, nil
			}
		}
	}
	var matches []LocationName
	for _, name := range AllLocationNames {
		if lowCaseInput != "" && strings.HasPrefix(strings.ToLower(string(name)), lowCaseInput) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", errors.New("Ambiguous destination " + inputName + "; type a little more of its name")
	}
	return "", errors.New("Can't find location " + inputName)
}

// The first alias is displayed on the map; all aliases accept spaces, hyphens
// and underscores interchangeably, without changing prefix ambiguity rules.
var locationAliases = map[LocationName][]string{
	Toolforge:     {"tools", "tf"},
	CloudVPS:      {"vps", "cloud vps", "horizon", "cloud"},
	Wikitech:      {"docs", "runbook", "runbooks"},
	Grafana:       {"graphs", "metrics", "dashboards"},
	Logstash:      {"logs", "errors"},
	Patchdemo:     {"demo", "patch demo"},
	Commons:       {"media", "uploads"},
	Translatewiki: {"i18n", "translate wiki", "translations"},
	BetaCluster:   {"beta", "beta cluster", "staging"},
	Codesearch:    {"search", "code search"},
	Deployments:   {"calendar", "train", "release calendar"},
	SAL:           {"adminlog", "admin log", "server admin log"},
	CommandLine:   {"cli", "terminal", "shell", "command line"},
	Phab:          {"phabricator", "phabricator tasks", "tasks"},
	Gerrit:        {"review", "code review", "reviews"},
	Gitlab:        {"gl", "git lab"},
	CI:            {"tests", "test", "pipeline", "pipelines", "continuous integration"},
	IRC:           {"chat", "libera", "liberachat", "libera chat", "#wikimedia-dev"},
	Telegram:      {"tg"},
	Discord:       {"dc"},
	WMFSlack:      {"slack", "wmf slack"},
	Email:         {"mail", "inbox", "e mail"},
	Meeting:       {"meet", "meetings", "google meet", "call"},
	Backports:     {"bp", "backport", "cherry pick", "cherrypick", "release branch"},
	Production:    {"prod", "deploy", "deploys", "deployment"},
	AFK:           {"break", "away", "tea"},
}

func normalizeDestination(input string) string {
	input = strings.NewReplacer("-", " ", "_", " ").Replace(strings.ToLower(input))
	return strings.Join(strings.Fields(input), " ")
}
