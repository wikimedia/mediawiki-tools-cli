package wikiapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"gitlab.wikimedia.org/repos/releng/cli/internal/diskcache"
)

// Discovery stores only explicitly fetched public schemas, never user responses.
type (
	Discovery struct{ Client *Client }
	Parameter struct {
		Name        string          `json:"name"`
		Type        json.RawMessage `json:"type"`
		Required    bool            `json:"required,omitempty"`
		Description string          `json:"description,omitempty"`
		Multi       bool            `json:"multi,omitempty"`
		Default     json.RawMessage `json:"default,omitempty"`
		Min         json.RawMessage `json:"min,omitempty"`
		Max         json.RawMessage `json:"max,omitempty"`
	}
)

type Module struct {
	ReadRights   bool        `json:"readrights,omitempty"`
	WriteRights  bool        `json:"writerights,omitempty"`
	MustBePosted bool        `json:"mustbeposted,omitempty"`
	HelpURLs     []string    `json:"helpurls,omitempty"`
	Name         string      `json:"name"`
	Path         string      `json:"path"`
	Prefix       string      `json:"prefix"`
	Parameters   []Parameter `json:"parameters"`
	Description  string      `json:"description,omitempty"`
}
type Site struct {
	General    json.RawMessage `json:"general"`
	Namespaces json.RawMessage `json:"namespaces"`
	Extensions json.RawMessage `json:"extensions"`
}
type RESTModule struct {
	ModuleID string `json:"moduleId"`
	Spec     string `json:"spec"`
}
type RESTIndex struct {
	Modules map[string]RESTModule `json:"modules"`
}
type RESTSpec struct {
	OpenAPI string `json:"openapi"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components json.RawMessage                       `json:"components,omitempty"`
}

func schemaCache[T any](endpoint, kind, name string) *diskcache.Cache[T] {
	installation := fmt.Sprintf("%x", sha256.Sum256([]byte(endpoint)))
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	return diskcache.New[T](filepath.Join(xdg.CacheHome, "mwcli", "wiki-api", installation, kind+"-"+key+".json"), 24*time.Hour)
}

func (d Discovery) CachedModule(name string) (Module, bool) {
	return schemaCache[Module](d.Client.Endpoint, "action", name).Peek()
}

func (d Discovery) CachedSite() (Site, bool) {
	return schemaCache[Site](d.Client.Endpoint, "site", "").Peek()
}

func (d Discovery) CachedRESTIndex() (RESTIndex, bool) {
	return schemaCache[RESTIndex](d.Client.Endpoint, "rest-index", "").Peek()
}

func (d Discovery) CachedREST(module string) (RESTSpec, bool) {
	return schemaCache[RESTSpec](d.Client.Endpoint, "rest", module).Peek()
}

// RefreshAction fetches paraminfo in bounded batches. An empty list discovers
// main, query, all top-level actions and all query submodules.
func (d Discovery) RefreshAction(ctx context.Context, names []string) ([]Module, error) {
	all := len(names) == 0
	if all {
		names = []string{"main", "query", "query+*"}
	}
	modules, err := d.fetchModules(ctx, names)
	if err != nil {
		return nil, err
	}
	if all {
		pending := []string{}
		for _, m := range modules {
			if m.Name == "main" {
				for _, p := range m.Parameters {
					if p.Name == "action" {
						for _, name := range p.Values() {
							if name != "query" {
								pending = append(pending, name)
							}
						}
					}
				}
			}
		}
		for len(pending) > 0 {
			n := min(50, len(pending))
			batch, e := d.fetchModules(ctx, pending[:n])
			if e != nil {
				return nil, e
			}
			modules = append(modules, batch...)
			pending = pending[n:]
		}
	}
	for _, m := range modules {
		key := m.Path
		if key == "" {
			key = m.Name
		}
		_, _ = schemaCache[Module](d.Client.Endpoint, "action", key).Refresh(ctx, func(context.Context) (Module, error) { return m, nil })
	}
	_, err = schemaCache[Site](d.Client.Endpoint, "site", "").Refresh(ctx, func(ctx context.Context) (Site, error) {
		data, e := d.Client.Action(ctx, http.MethodGet, url.Values{"action": {"query"}, "meta": {"siteinfo"}, "siprop": {"general|namespaces|extensions"}})
		var response struct {
			Query Site `json:"query"`
		}
		if e == nil {
			e = json.Unmarshal(data, &response)
		}
		return response.Query, e
	})
	return modules, err
}

func (d Discovery) fetchModules(ctx context.Context, names []string) ([]Module, error) {
	data, err := d.Client.Action(ctx, http.MethodGet, url.Values{"action": {"paraminfo"}, "modules": {strings.Join(names, "|")}, "helpformat": {"wikitext"}})
	if err != nil {
		return nil, err
	}
	var result struct {
		Paraminfo struct {
			Modules []Module `json:"modules"`
		} `json:"paraminfo"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if len(result.Paraminfo.Modules) == 0 {
		return nil, fmt.Errorf("wiki did not return parameter information")
	}
	return result.Paraminfo.Modules, nil
}

func (p Parameter) Values() []string {
	var values []json.RawMessage
	if json.Unmarshal(p.Type, &values) != nil {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			result = append(result, text)
		} else {
			result = append(result, string(raw))
		}
	}
	return result
}

// RefreshREST consumes same-installation OpenAPI documents only. A wiki without
// specs remains usable through raw REST requests.
func (d Discovery) RefreshREST(ctx context.Context) ([]string, error) {
	index, err := schemaCache[RESTIndex](d.Client.Endpoint, "rest-index", "").Refresh(ctx, func(ctx context.Context) (RESTIndex, error) {
		data, e := d.Client.REST(ctx, http.MethodGet, "specs/v0/discovery", nil, nil, nil)
		var v RESTIndex
		if e == nil {
			e = json.Unmarshal(data, &v)
		}
		if e == nil && len(v.Modules) == 0 {
			e = fmt.Errorf("wiki has no REST discovery modules")
		}
		return v, e
	})
	if err != nil {
		// Older Wikibase deployments can serve this before generic discovery exists.
		spec, e := d.fetchRESTSpec(ctx, "wikibase/v1/openapi.json")
		if e != nil {
			return nil, fmt.Errorf("REST discovery unavailable; raw REST requests still work: %w", err)
		}
		_, e = schemaCache[RESTSpec](d.Client.Endpoint, "rest", "wikibase/v1").Refresh(ctx, func(context.Context) (RESTSpec, error) { return spec, nil })
		if e != nil {
			return nil, e
		}
		index = RESTIndex{Modules: map[string]RESTModule{"wikibase/v1": {ModuleID: "wikibase/v1", Spec: "wikibase/v1/openapi.json"}}}
		_, _ = schemaCache[RESTIndex](d.Client.Endpoint, "rest-index", "").Refresh(ctx, func(context.Context) (RESTIndex, error) { return index, nil })
		return []string{"wikibase/v1"}, nil
	}
	names := []string{}
	for key, mod := range index.Modules {
		route := mod.Spec
		if route == "" {
			route = "specs/v0/module/" + key
			if key == "" {
				route += "-"
			}
		}
		// Discovery may use absolute spec URLs. Accept only this installation;
		// never follow a server-supplied URL to another origin or script path.
		if u, e := url.Parse(route); e == nil && u.IsAbs() {
			prefix := strings.TrimSuffix(d.Client.Endpoint, "api.php") + "rest.php/"
			if !strings.HasPrefix(route, prefix) {
				continue
			}
			route = strings.TrimPrefix(route, prefix)
		}
		spec, e := d.fetchRESTSpec(ctx, route)
		if e != nil {
			continue
		}
		_, _ = schemaCache[RESTSpec](d.Client.Endpoint, "rest", key).Refresh(ctx, func(context.Context) (RESTSpec, error) { return spec, nil })
		names = append(names, key)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("wiki did not provide usable REST schemas; raw REST requests still work")
	}
	return names, nil
}

func (d Discovery) fetchRESTSpec(ctx context.Context, route string) (RESTSpec, error) {
	data, err := d.Client.REST(ctx, http.MethodGet, route, nil, nil, nil)
	var spec RESTSpec
	if err == nil {
		err = json.Unmarshal(data, &spec)
	}
	if err == nil && (spec.OpenAPI == "" || spec.Paths == nil) {
		err = fmt.Errorf("not an OpenAPI document")
	}
	return spec, err
}
