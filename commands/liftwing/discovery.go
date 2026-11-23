package liftwing

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type model struct {
	Name          string         `json:"name"`
	Endpoint      string         `json:"endpoint"`
	Specification string         `json:"specification"`
	Operation     map[string]any `json:"operation,omitempty"`
	path          string
	route         map[string]any
}

type specification struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

func discover(ctx context.Context) ([]model, error) {
	data, err := request(ctx, http.MethodGet, specURL, nil)
	if err != nil {
		return nil, err
	}
	var spec specification
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("decode Lift Wing specification: %w", err)
	}
	models := make([]model, 0, len(spec.Paths))
	for path, route := range spec.Paths {
		name, ok := strings.CutPrefix(path, modelPrefix)
		if !ok || !strings.HasSuffix(name, ":predict") {
			continue
		}
		name = strings.TrimSuffix(name, ":predict")
		if strings.Contains(name, "/") || name == "" {
			continue
		}
		models = append(models, model{Name: name, Endpoint: baseURL + path, Specification: specURL, path: path, route: route})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("Lift Wing specification contains no prediction endpoints")
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	return models, nil
}

var modelName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// The public specification documents Revscoring families as {wiki}-MODEL.
// A matching family documents the route, not availability on every wiki.
func findModel(ctx context.Context, name string) (model, error) {
	if !modelName.MatchString(name) {
		return model{}, fmt.Errorf("invalid model name %q; use liftwing models list", name)
	}
	models, err := discover(ctx)
	if err != nil {
		return model{}, err
	}
	for _, m := range models {
		matches := m.Name == name
		if suffix, ok := strings.CutPrefix(m.Name, "{wiki}"); ok {
			wiki, found := strings.CutSuffix(name, suffix)
			matches = found && wiki != "" && modelName.MatchString(wiki)
		}
		if matches {
			m.Name = name
			m.Endpoint = baseURL + modelPrefix + name + ":predict"
			return m, nil
		}
	}
	return model{}, fmt.Errorf("model %q has no documented prediction endpoint; use liftwing models list", name)
}

func modelInfo(ctx context.Context, name string) (model, error) {
	m, err := findModel(ctx, name)
	if err != nil {
		return model{}, err
	}
	route := m.route
	if ref, ok := route["$ref"].(string); ok {
		base, _ := url.Parse(specURL)
		reference, err := url.Parse(ref)
		if err != nil {
			return model{}, fmt.Errorf("invalid model specification reference: %w", err)
		}
		resolved := base.ResolveReference(reference)
		if resolved.Scheme != base.Scheme || resolved.Host != base.Host || !strings.HasPrefix(resolved.Path, "/service/lw/specs/") {
			return model{}, fmt.Errorf("model specification references a URL outside Lift Wing specifications")
		}
		// Path item references in the public spec point at the same path in
		// per-model files. Reject a changed reference instead of reading a
		// different operation silently.
		pointer := "/paths/" + strings.ReplaceAll(strings.ReplaceAll(m.path, "~", "~0"), "/", "~1")
		if resolved.Fragment != pointer {
			return model{}, fmt.Errorf("unsupported model specification reference %q", ref)
		}
		resolved.Fragment = ""
		m.Specification = resolved.String()
		data, err := request(ctx, http.MethodGet, m.Specification, nil)
		if err != nil {
			return model{}, err
		}
		var spec specification
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return model{}, fmt.Errorf("decode model specification: %w", err)
		}
		route = spec.Paths[m.path]
	}
	operation, ok := route["post"].(map[string]any)
	if !ok {
		return model{}, fmt.Errorf("model specification has no POST operation")
	}
	m.Operation = operation
	return m, nil
}
