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
		Submodules  map[string]string `json:"submodules,omitempty"`
		Name        string            `json:"name"`
		Type        json.RawMessage   `json:"type"`
		Required    bool              `json:"required,omitempty"`
		Description string            `json:"description,omitempty"`
		Multi       bool              `json:"multi,omitempty"`
		Default     json.RawMessage   `json:"default,omitempty"`
		Min         json.RawMessage   `json:"min,omitempty"`
		Max         json.RawMessage   `json:"max,omitempty"`
	}
)

type ActionIndex struct {
	Modules []string `json:"modules"`
}

type Module struct {
	Missing      bool        `json:"missing,omitempty"`
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

func (d Discovery) CachedActionIndex() (ActionIndex, bool) {
	return schemaCache[ActionIndex](d.Client.Endpoint, "action-index", "").Peek()
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

// RefreshAction fetches paraminfo in bounded batches. With no names it follows
// all advertised submodule mappings, including nested extension modules.
func (d Discovery) RefreshAction(ctx context.Context, names []string) ([]Module, error) {
	all := len(names) == 0
	if all {
		names = []string{"main", "query"}
	}
	pending := append([]string(nil), names...)
	seen := map[string]bool{}
	modules := []Module{}
	for len(pending) > 0 {
		batch := []string{}
		for len(pending) > 0 && len(batch) < 50 {
			name := pending[0]
			pending = pending[1:]
			if !seen[name] {
				seen[name] = true
				batch = append(batch, name)
			}
		}
		if len(batch) == 0 {
			continue
		}
		if len(seen) > 10000 {
			return nil, fmt.Errorf("API discovery exceeded 10000 modules")
		}
		discovered, err := d.fetchModules(ctx, batch)
		if err != nil {
			return nil, err
		}
		modules = append(modules, discovered...)
		if all {
			for _, m := range discovered {
				for _, p := range m.Parameters {
					for _, child := range p.Submodules {
						if !seen[child] {
							pending = append(pending, child)
						}
					}
				}
			}
		}
	}
	index := ActionIndex{}
	if !all {
		index, _ = d.CachedActionIndex()
	}
	indexed := map[string]bool{}
	for _, name := range index.Modules {
		indexed[name] = true
	}
	for _, m := range modules {
		key := m.Path
		if key == "" {
			key = m.Name
		}
		indexed[key] = true
		_, _ = schemaCache[Module](d.Client.Endpoint, "action", key).Refresh(ctx, func(context.Context) (Module, error) { return m, nil })
	}
	index.Modules = nil
	for name := range indexed {
		index.Modules = append(index.Modules, name)
	}
	sort.Strings(index.Modules)
	_, _ = schemaCache[ActionIndex](d.Client.Endpoint, "action-index", "").Refresh(ctx, func(context.Context) (ActionIndex, error) { return index, nil })
	_, err := schemaCache[Site](d.Client.Endpoint, "site", "").Refresh(ctx, func(ctx context.Context) (Site, error) {
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
	for _, m := range result.Paraminfo.Modules {
		if m.Missing {
			return nil, fmt.Errorf("unknown API module %q", m.Path)
		}
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

// RefreshREST consumes advertised same-installation schemas and separately
// probes the legacy Wikibase spec, which older deployments do not advertise.
// Only successfully fetched schemas enter the index used by completion.
func (d Discovery) RefreshREST(ctx context.Context) ([]string, error) {
	data, discoveryErr := d.Client.REST(ctx, http.MethodGet, "specs/v0/discovery", nil, nil, nil)
	advertised := RESTIndex{}
	if discoveryErr == nil {
		discoveryErr = json.Unmarshal(data, &advertised)
	}
	successful := RESTIndex{Modules: map[string]RESTModule{}}
	for key, mod := range advertised.Modules {
		route := mod.Spec
		if route == "" {
			route = "specs/v0/module/" + key
			if key == "" {
				route += "-"
			}
		}
		route, ok := d.specRoute(route)
		if !ok {
			continue
		}
		spec, err := d.fetchRESTSpec(ctx, route)
		if err != nil {
			continue
		}
		if _, err = schemaCache[RESTSpec](d.Client.Endpoint, "rest", key).Refresh(ctx, func(context.Context) (RESTSpec, error) { return spec, nil }); err != nil {
			continue
		}
		mod.Spec = route
		successful.Modules[key] = mod
	}
	if _, ok := successful.Modules["wikibase/v1"]; !ok && ctx.Err() == nil {
		route := "wikibase/v1/openapi.json"
		if spec, err := d.fetchRESTSpec(ctx, route); err == nil {
			_, _ = schemaCache[RESTSpec](d.Client.Endpoint, "rest", "wikibase/v1").Refresh(ctx, func(context.Context) (RESTSpec, error) { return spec, nil })
			successful.Modules["wikibase/v1"] = RESTModule{ModuleID: "wikibase/v1", Spec: route}
		}
	}
	names := []string{}
	for name := range successful.Modules {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		if discoveryErr != nil {
			return nil, fmt.Errorf("REST discovery unavailable; raw REST requests still work: %w", discoveryErr)
		}
		return nil, fmt.Errorf("wiki did not provide usable REST schemas; raw REST requests still work")
	}
	_, _ = schemaCache[RESTIndex](d.Client.Endpoint, "rest-index", "").Refresh(ctx, func(context.Context) (RESTIndex, error) { return successful, nil })
	return names, nil
}

func (d Discovery) specRoute(route string) (string, bool) {
	u, err := url.Parse(route)
	if err != nil {
		return "", false
	}
	prefix := strings.TrimSuffix(d.Client.Endpoint, "api.php") + "rest.php/"
	if u.IsAbs() {
		if !strings.HasPrefix(route, prefix) {
			return "", false
		}
		route = strings.TrimPrefix(route, prefix)
	} else if strings.HasPrefix(route, "/") {
		base, _ := url.Parse(prefix)
		if !strings.HasPrefix(route, base.Path) {
			return "", false
		}
		route = strings.TrimPrefix(route, base.Path)
	}
	_, err = d.Client.RESTPath(route)
	return route, err == nil
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
