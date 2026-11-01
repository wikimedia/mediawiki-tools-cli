package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var siteIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// EntitySitelinks retains native fields, including URLs and badges. Empty native
// arrays from older Wikibase installations are normalized to an empty object.
func EntitySitelinks(entity Entity) (map[string]json.RawMessage, error) {
	links := map[string]json.RawMessage{}
	raw := entity["sitelinks"]
	if len(raw) == 0 || string(raw) == "[]" {
		return links, nil
	}
	if err := json.Unmarshal(raw, &links); err != nil {
		return nil, err
	}
	if links == nil {
		links = map[string]json.RawMessage{}
	}
	return links, nil
}

// Badge omission preserves server-side badges; explicit replacement (including
// empty) follows wbsetsitelink's documented semantics:
// https://www.wikidata.org/wiki/Help:API:wbsetsitelink
func (c *Client) PlanSitelink(ctx context.Context, id, site, title string, remove bool, badges []string, replaceBadges bool, expected int64) (*WikibaseEditPlan, error) {
	if !siteIDPattern.MatchString(site) {
		return nil, fmt.Errorf("invalid site ID %q", site)
	}
	if !remove && strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("sitelink title cannot be empty; use remove")
	}
	if remove && (replaceBadges || len(badges) > 0) {
		return nil, fmt.Errorf("remove cannot set badges")
	}
	if !replaceBadges && len(badges) > 0 {
		return nil, fmt.Errorf("badge replacement must be explicit")
	}
	canonical, err := EntityID("item", id)
	if err != nil {
		return nil, err
	}
	unique := []string{}
	seen := map[string]bool{}
	for _, badge := range badges {
		badge, err = EntityID("item", badge)
		if err != nil {
			return nil, err
		}
		if !seen[badge] {
			unique = append(unique, badge)
			seen[badge] = true
		}
	}
	entity, revision, err := c.EntityForEdit(ctx, "item", canonical, "info|sitelinks", expected)
	if err != nil {
		return nil, err
	}
	links, err := EntitySitelinks(entity)
	if err != nil {
		return nil, err
	}
	if remove {
		if _, ok := links[site]; !ok {
			return nil, fmt.Errorf("item %s has no sitelink for %s", canonical, site)
		}
		title = ""
	}
	fields := url.Values{"action": {"wbsetsitelink"}, "id": {canonical}, "linksite": {site}, "linktitle": {title}}
	change := map[string]interface{}{"site": site, "title": title, "remove": remove, "badge_mode": "preserve"}
	if replaceBadges {
		fields.Set("badges", apiList(unique))
		change["badge_mode"] = "replace"
		change["badges"] = unique
	}
	return &WikibaseEditPlan{Entity: canonical, BaseRevision: revision, Action: "wbsetsitelink", Change: change, fields: fields}, nil
}
