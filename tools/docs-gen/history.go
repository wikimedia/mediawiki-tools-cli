package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
)

const publicationSummary = "Publish a Wikimedia CLI documentation"

func automaticPublication(config configuration, title string, page snapshot) bool {
	if page.Comment == publicationSummary || strings.HasPrefix(page.Comment, publicationSummary+" (") {
		_, valid := unmark(page.Text)
		return valid
	}
	return config.LegacyPublisher != "" && config.LegacySummary != "" &&
		strings.HasPrefix(title, config.ReferenceRoot+"/") &&
		page.User == config.LegacyPublisher && page.Comment == config.LegacySummary && !strings.Contains(page.Text, markerPrefix)
}

func revisionSnapshot(title string, revision wikiRevision) (snapshot, error) {
	slot, readable := revision.Slots["main"]
	if !readable || slot.Content == nil || slot.Hidden || revision.ID <= 0 {
		return snapshot{}, fmt.Errorf("cannot read revision %d of %s", revision.ID, title)
	}
	return snapshot{Revision: revision.ID, Text: *slot.Content, User: revision.User, Comment: revision.Comment}, nil
}

// Search only revisions at or before the current snapshot. Later edits are
// protected by baserevid at publication time, not mixed into this comparison.
func lastPublication(client *mwclient.Client, config configuration, title string, current snapshot) (*snapshot, error) {
	if current.Missing {
		return nil, nil
	}
	if automaticPublication(config, title, current) {
		return &current, nil
	}
	values := params.Values{"action": "query", "formatversion": "2", "prop": "revisions", "rvprop": "ids|user|comment", "rvlimit": "500", "titles": title, "rvstartid": strconv.FormatInt(current.Revision, 10)}
	seen := map[string]bool{}
	for {
		response, err := client.Get(values)
		if err != nil {
			return nil, err
		}
		data, err := response.Marshal()
		if err != nil {
			return nil, err
		}
		var reply struct {
			Continue map[string]string `json:"continue"`
			Query    struct {
				Pages []wikiPage `json:"pages"`
			} `json:"query"`
		}
		if err := json.Unmarshal(data, &reply); err != nil {
			return nil, err
		}
		if len(reply.Query.Pages) != 1 {
			return nil, fmt.Errorf("missing revision history for %s", title)
		}
		for _, revision := range reply.Query.Pages[0].Revisions {
			// Read content only for potential automatic publications, not every
			// intervening typo fix, revert or suppressed vandalism revision.
			modern := revision.Comment == publicationSummary || strings.HasPrefix(revision.Comment, publicationSummary+" (")
			legacy := config.LegacyPublisher != "" && config.LegacySummary != "" && strings.HasPrefix(title, config.ReferenceRoot+"/") && revision.User == config.LegacyPublisher && revision.Comment == config.LegacySummary
			if !modern && !legacy {
				continue
			}
			candidate := current
			if revision.ID != current.Revision {
				var err error
				candidate, err = fetchRevision(client, title, revision.ID)
				if err != nil {
					return nil, err
				}
			}
			if automaticPublication(config, title, candidate) {
				return &candidate, nil
			}
		}
		continuation := reply.Continue["rvcontinue"]
		if continuation == "" {
			break
		}
		if seen[continuation] {
			return nil, fmt.Errorf("repeated revision-history continuation for %s", title)
		}
		seen[continuation] = true
		for key, value := range reply.Continue {
			values[key] = value
		}
	}
	// Guides adopted from the wiki may not have an automatic publication yet.
	// Until the first release, compare against their explicitly reviewed revision.
	reviewed, ok := config.Reviewed[title]
	if !ok {
		return nil, nil
	}
	if reviewed.SHA256 == digest(normalize(current.Text)) {
		baseline := current
		baseline.Revision = reviewed.Revision
		return &baseline, nil
	}
	baseline, err := fetchRevision(client, title, reviewed.Revision)
	if err != nil {
		return nil, err
	}
	if baseline.Revision != reviewed.Revision || digest(normalize(baseline.Text)) != reviewed.SHA256 {
		return nil, fmt.Errorf("reviewed revision does not match saved content for %s", title)
	}
	return &baseline, nil
}

func fetchRevision(client *mwclient.Client, title string, id int64) (snapshot, error) {
	response, err := client.Get(params.Values{"action": "query", "formatversion": "2", "prop": "revisions", "rvprop": "ids|content|user|comment", "rvslots": "main", "revids": strconv.FormatInt(id, 10)})
	if err != nil {
		return snapshot{}, err
	}
	data, err := response.Marshal()
	if err != nil {
		return snapshot{}, err
	}
	var reply struct {
		Query struct {
			Pages []wikiPage `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return snapshot{}, err
	}
	if len(reply.Query.Pages) != 1 || len(reply.Query.Pages[0].Revisions) != 1 {
		return snapshot{}, fmt.Errorf("missing revision %d of %s", id, title)
	}
	result, err := revisionSnapshot(title, reply.Query.Pages[0].Revisions[0])
	if err != nil {
		return snapshot{}, err
	}
	if result.Revision != id {
		return snapshot{}, fmt.Errorf("unexpected revision for %s", title)
	}
	return result, nil
}

func documentationBody(text string) string {
	body, _ := unmark(text)
	return normalize(body)
}
