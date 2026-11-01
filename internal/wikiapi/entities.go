package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	entityIDPattern = regexp.MustCompile(`^[QP][1-9][0-9]*$`)
	languagePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
)

// Entity is native Wikibase Action JSON. Unknown fields survive reads/export.
type Entity map[string]json.RawMessage

func (e Entity) Revision() (int64, error) {
	var revision int64
	if err := json.Unmarshal(e["lastrevid"], &revision); err != nil || revision <= 0 {
		return 0, fmt.Errorf("entity has no usable lastrevid")
	}
	return revision, nil
}

func EntityID(kind, id string) (string, error) {
	id = strings.ToUpper(id)
	if !entityIDPattern.MatchString(id) || (kind == "item" && !strings.HasPrefix(id, "Q")) || (kind == "property" && !strings.HasPrefix(id, "P")) {
		return "", fmt.Errorf("invalid %s ID %q", kind, id)
	}
	return id, nil
}

func ValidateLanguages(languages []string) error {
	for _, language := range languages {
		if !languagePattern.MatchString(language) {
			return fmt.Errorf("invalid language code %q", language)
		}
	}
	return nil
}

func (c *Client) Entity(ctx context.Context, kind, id string, languages []string, props string, fallback bool) (Entity, error) {
	id, err := EntityID(kind, id)
	if err != nil {
		return nil, err
	}
	if err = ValidateLanguages(languages); err != nil {
		return nil, err
	}
	fields := url.Values{"action": {"wbgetentities"}, "ids": {id}}
	if len(languages) > 0 {
		fields.Set("languages", strings.Join(languages, "|"))
	}
	if props != "" {
		fields.Set("props", props)
	}
	if fallback {
		fields.Set("languagefallback", "1")
	}
	data, err := c.Action(ctx, http.MethodGet, fields)
	if err != nil {
		return nil, err
	}
	var result struct {
		Entities map[string]Entity `json:"entities"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	entity, ok := result.Entities[id]
	if !ok {
		return nil, fmt.Errorf("wiki did not return entity %s", id)
	}
	if _, missing := entity["missing"]; missing {
		return nil, fmt.Errorf("entity %s does not exist", id)
	}
	var actual string
	_ = json.Unmarshal(entity["type"], &actual)
	if actual != "" && actual != kind {
		return nil, fmt.Errorf("entity %s is a %s, not a %s", id, actual, kind)
	}
	return entity, nil
}

type EntitySearchResult struct {
	ID          string `json:"id"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

func (c *Client) SearchEntities(ctx context.Context, kind, query, language string, limit int) ([]EntitySearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}
	if err := ValidateLanguages([]string{language}); err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("--limit must be nonnegative")
	}
	fields := url.Values{"action": {"wbsearchentities"}, "search": {query}, "language": {language}, "uselang": {language}, "type": {kind}, "limit": {"50"}}
	found := []EntitySearchResult{}
	seen := map[string]bool{}
	cursors := map[string]bool{}
	for {
		data, err := c.Action(ctx, http.MethodGet, fields)
		if err != nil {
			return nil, err
		}
		var result struct {
			Search   []EntitySearchResult `json:"search"`
			Continue json.RawMessage      `json:"search-continue"`
		}
		if err = json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		for _, entity := range result.Search {
			if !seen[entity.ID] {
				seen[entity.ID] = true
				found = append(found, entity)
				if limit > 0 && len(found) >= limit {
					return found, nil
				}
			}
		}
		if len(result.Continue) == 0 || string(result.Continue) == "null" {
			return found, nil
		}
		var cursor string
		if json.Unmarshal(result.Continue, &cursor) != nil {
			cursor = string(result.Continue)
		}
		if cursors[cursor] {
			return nil, fmt.Errorf("wiki repeated its entity search cursor")
		}
		cursors[cursor] = true
		fields.Set("continue", cursor)
	}
}

// EditOptions applies to individual entity edits only. Baserevid is mandatory
// for updates at this layer; command helpers obtain one by reading the entity.
type EditOptions struct {
	Summary      string
	BaseRevision int64
	Bot          bool
}

func (c *Client) editFields(options EditOptions, fields url.Values) (url.Values, error) {
	if options.BaseRevision < 0 {
		return nil, fmt.Errorf("--baserevid must be nonnegative")
	}
	fields.Set("summary", options.Summary)
	fields.Set("maxlag", "5")
	if options.BaseRevision > 0 {
		fields.Set("baserevid", strconv.FormatInt(options.BaseRevision, 10))
	}
	if options.Bot {
		fields.Set("bot", "1")
	}
	return fields, nil
}

func (c *Client) edit(ctx context.Context, fields url.Values) (json.RawMessage, error) {
	tokens, err := c.Action(ctx, http.MethodGet, url.Values{"action": {"query"}, "meta": {"tokens"}, "type": {"csrf"}})
	if err != nil {
		return nil, err
	}
	var result struct {
		Query struct {
			Tokens struct {
				CSRF string `json:"csrftoken"`
			} `json:"tokens"`
		} `json:"query"`
	}
	if json.Unmarshal(tokens, &result) != nil || result.Query.Tokens.CSRF == "" {
		return nil, fmt.Errorf("wiki returned no CSRF token")
	}
	fields.Set("token", result.Query.Tokens.CSRF)
	return c.Action(ctx, http.MethodPost, fields)
}

func ValidateEntityCreate(kind string, data []byte) (Entity, error) {
	if kind != "item" && kind != "property" {
		return nil, fmt.Errorf("unsupported entity type %q", kind)
	}
	var entity Entity
	if err := json.Unmarshal(data, &entity); err != nil || entity == nil {
		return nil, fmt.Errorf("entity input must be a JSON object")
	}
	if _, exists := entity["id"]; exists {
		return nil, fmt.Errorf("create input must not contain an existing entity id")
	}
	if raw, exists := entity["type"]; exists {
		var actual string
		if json.Unmarshal(raw, &actual) != nil || actual != kind {
			return nil, fmt.Errorf("input type must be %q", kind)
		}
	}
	if kind == "property" {
		var datatype string
		if json.Unmarshal(entity["datatype"], &datatype) != nil || datatype == "" {
			return nil, fmt.Errorf("property creation requires a datatype")
		}
	}
	return entity, nil
}

func (c *Client) CreateEntity(ctx context.Context, kind string, data []byte, options EditOptions) (json.RawMessage, error) {
	if _, err := ValidateEntityCreate(kind, data); err != nil {
		return nil, err
	}
	if options.BaseRevision != 0 {
		return nil, fmt.Errorf("--baserevid applies to existing entities only")
	}
	fields, err := c.editFields(options, url.Values{"action": {"wbeditentity"}, "new": {kind}, "data": {string(data)}})
	if err != nil {
		return nil, err
	}
	return c.edit(ctx, fields)
}

func TermProperty(term string) (string, error) {
	switch term {
	case "label":
		return "labels", nil
	case "description":
		return "descriptions", nil
	case "alias":
		return "aliases", nil
	}
	return "", fmt.Errorf("unsupported entity term %q", term)
}

func ValidateTermChange(term, operation, language string, values []string) error {
	if _, err := TermProperty(term); err != nil {
		return err
	}
	if err := ValidateLanguages([]string{language}); err != nil {
		return err
	}
	if term == "alias" {
		if operation != "set" && operation != "add" && operation != "remove" {
			return fmt.Errorf("unsupported alias operation")
		}
		if len(values) == 0 {
			return fmt.Errorf("provide at least one alias")
		}
	} else {
		if operation != "set" && operation != "remove" {
			return fmt.Errorf("unsupported term operation")
		}
		if operation == "set" && (len(values) != 1 || strings.TrimSpace(values[0]) == "") {
			return fmt.Errorf("provide exactly one nonempty value")
		}
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\x00\x1f\r\n") {
			return fmt.Errorf("term values must be nonempty single lines without control separators")
		}
	}
	return nil
}

func (c *Client) ChangeTerm(ctx context.Context, kind, id, term, operation, language string, values []string, options EditOptions) (json.RawMessage, error) {
	id, err := EntityID(kind, id)
	if err != nil {
		return nil, err
	}
	if err = ValidateTermChange(term, operation, language, values); err != nil {
		return nil, err
	}
	if options.BaseRevision <= 0 {
		return nil, fmt.Errorf("term updates require a positive base revision")
	}
	fields := url.Values{"id": {id}, "language": {language}}
	if term == "alias" {
		fields.Set("action", "wbsetaliases")
		fields.Set(operation, apiList(values))
	} else {
		fields.Set("action", "wbset"+term)
		if operation == "set" {
			fields.Set("value", values[0])
		} else {
			fields.Set("value", "")
		}
	}
	fields, err = c.editFields(options, fields)
	if err != nil {
		return nil, err
	}
	return c.edit(ctx, fields)
}
