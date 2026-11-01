package wikiapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// queryContinuation follows the entire opaque Action continuation map, removes
// obsolete cursor keys and detects a server returning the same cursor twice.
type queryContinuation struct {
	keys []string
	seen map[string]bool
}

func (c *queryContinuation) advance(fields url.Values, next map[string]json.RawMessage) (bool, error) {
	if len(next) == 0 {
		return false, nil
	}
	values := make(url.Values)
	for key, raw := range next {
		if strings.TrimSpace(string(raw)) == "null" {
			return false, fmt.Errorf("wiki returned an invalid continuation value for %s", key)
		}
		if fields.Has(key) {
			previous := false
			for _, cursorKey := range c.keys {
				if key == cursorKey {
					previous = true
					break
				}
			}
			if !previous {
				return false, fmt.Errorf("wiki continuation would overwrite fixed parameter %s", key)
			}
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			var number json.Number
			if json.Unmarshal(raw, &number) != nil || strings.TrimSpace(string(raw)) == "null" {
				return false, fmt.Errorf("wiki returned an invalid continuation value for %s", key)
			}
			value = number.String()
		}
		values.Set(key, value)
	}
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	marker := values.Encode()
	if c.seen[marker] {
		return false, fmt.Errorf("wiki repeated its continuation cursor")
	}
	c.seen[marker] = true
	for _, key := range c.keys {
		fields.Del(key)
	}
	c.keys = nil
	for key, values := range values {
		fields[key] = values
		c.keys = append(c.keys, key)
	}
	return true, nil
}
