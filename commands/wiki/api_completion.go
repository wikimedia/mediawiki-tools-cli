package wiki

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func completeAPIActions(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	d, err := completionDiscovery()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	m, ok := d.CachedModule("main")
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	for _, p := range m.Parameters {
		if p.Name == "action" {
			return matchAPIValues(p.Values(), prefix), cobra.ShellCompDirectiveNoFileComp
		}
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func completeAPIModules(cmd *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	values, _ := completeAPIActions(cmd, nil, "")
	values = append(values, "main")
	if d, err := completionDiscovery(); err == nil {
		if m, ok := d.CachedModule("query"); ok {
			for _, p := range m.Parameters {
				if p.Name == "list" || p.Name == "prop" || p.Name == "meta" || p.Name == "generator" {
					for _, v := range p.Values() {
						values = append(values, "query+"+v)
					}
				}
			}
		}
	}
	return matchAPIValues(values, prefix), cobra.ShellCompDirectiveNoFileComp
}

func completeAPIFields(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	d, err := completionDiscovery()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := []string{"main", args[0]}
	fields, _ := cmd.Flags().GetStringArray("field")
	if args[0] == "query" {
		for _, f := range fields {
			k, v, _ := strings.Cut(f, "=")
			if k == "list" || k == "prop" || k == "meta" || k == "generator" {
				for _, part := range strings.Split(v, "|") {
					names = append(names, "query+"+part)
				}
			}
		}
	}
	parameters := map[string]wikiapi.Parameter{}
	for _, name := range names {
		if m, ok := d.CachedModule(name); ok {
			for _, p := range m.Parameters {
				parameters[m.Prefix+p.Name] = p
			}
		}
	}
	// Site namespaces are useful even when the schema reports a numeric type.
	if site, ok := d.CachedSite(); ok {
		var namespaces map[string]struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(site.Namespaces, &namespaces) == nil {
			for key, p := range parameters {
				if strings.Contains(key, "namespace") {
					values := p.Values()
					for id := range namespaces {
						values = append(values, id)
					}
					p.Type, _ = json.Marshal(values)
					parameters[key] = p
				}
			}
		}
	}
	return completeParameters(parameters, prefix), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

func completeParameters(parameters map[string]wikiapi.Parameter, prefix string) []string {
	key, value, hasValue := strings.Cut(prefix, "=")
	values := []string{}
	if hasValue {
		if p, ok := parameters[key]; ok {
			head, tail := "", value
			if n := strings.LastIndex(value, "|"); n >= 0 {
				head, tail = value[:n+1], value[n+1:]
			}
			for _, v := range matchAPIValues(p.Values(), tail) {
				values = append(values, key+"="+head+v)
			}
		}
	} else {
		for key, p := range parameters {
			if strings.HasPrefix(key, prefix) {
				values = append(values, key+"=\t"+completionDescription(p.Description))
			}
		}
	}
	sort.Strings(values)
	return values
}

func matchAPIValues(values []string, prefix string) []string {
	set := map[string]bool{}
	for _, v := range values {
		if strings.HasPrefix(v, prefix) {
			set[v] = true
		}
	}
	result := []string{}
	for v := range set {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}

func restSpecs(d wikiapi.Discovery) map[string]wikiapi.RESTSpec {
	result := map[string]wikiapi.RESTSpec{}
	index, ok := d.CachedRESTIndex()
	if !ok {
		return result
	}
	for module := range index.Modules {
		if spec, ok := d.CachedREST(module); ok {
			result[module] = spec
		}
	}
	return result
}

func restRoute(d wikiapi.Discovery, module string, spec wikiapi.RESTSpec, p string) string {
	base := strings.TrimSuffix(d.Client.Endpoint, "api.php") + "rest.php"
	if len(spec.Servers) > 0 {
		server := spec.Servers[0].URL
		if strings.HasPrefix(server, base) {
			module = strings.TrimPrefix(server, base)
		} else if strings.HasPrefix(server, "/") {
			endpoint, _ := url.Parse(d.Client.Endpoint)
			script := strings.TrimSuffix(endpoint.Path, "api.php") + "rest.php"
			if server == script || strings.HasPrefix(server, script+"/") {
				module = strings.TrimPrefix(server, script)
			}
		}
	}
	return "/" + strings.Trim(strings.Trim(module, "/")+"/"+strings.TrimLeft(p, "/"), "/")
}

func completeRESTPaths(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return matchAPIValues([]string{"get", "head", "post", "put", "patch", "delete", "options"}, prefix), cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) != 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	d, err := completionDiscovery()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	paths := []string{}
	for module, spec := range restSpecs(d) {
		for p, ops := range spec.Paths {
			if _, ok := ops[strings.ToLower(args[0])]; ok {
				paths = append(paths, restRoute(d, module, spec, p))
			}
		}
	}
	return matchAPIValues(paths, prefix), cobra.ShellCompDirectiveNoFileComp
}

func completeRESTFields(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	d, err := completionDiscovery()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	parameters := map[string]wikiapi.Parameter{}
	for module, spec := range restSpecs(d) {
		for path, ops := range spec.Paths {
			if !restTemplateMatches(restRoute(d, module, spec, path), args[1]) {
				continue
			}
			var op struct {
				Parameters []json.RawMessage `json:"parameters"`
			}
			_ = json.Unmarshal(ops[strings.ToLower(args[0])], &op)
			var shared []json.RawMessage
			_ = json.Unmarshal(ops["parameters"], &shared)
			op.Parameters = append(op.Parameters, shared...)
			for _, raw := range op.Parameters {
				raw = restLocalRef(spec, raw)
				var p struct {
					Name        string          `json:"name"`
					In          string          `json:"in"`
					Description string          `json:"description"`
					Schema      json.RawMessage `json:"schema"`
				}
				_ = json.Unmarshal(raw, &p)
				if p.In == "query" {
					var schema struct {
						Enum json.RawMessage `json:"enum"`
					}
					_ = json.Unmarshal(restLocalRef(spec, p.Schema), &schema)
					parameters[p.Name] = wikiapi.Parameter{Name: p.Name, Description: p.Description, Type: schema.Enum}
				}
			}
		}
	}
	return completeParameters(parameters, prefix), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

func restTemplateMatches(template, route string) bool {
	target, err := url.Parse(route)
	if err != nil {
		return false
	}
	a, b := strings.Split(strings.Trim(template, "/"), "/"), strings.Split(strings.Trim(target.Path, "/"), "/")
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && !(strings.HasPrefix(a[i], "{") && strings.HasSuffix(a[i], "}")) {
			return false
		}
	}
	return true
}

// Resolve local OpenAPI references only. Completion never fetches remote refs.
func restLocalRef(spec wikiapi.RESTSpec, raw json.RawMessage) json.RawMessage {
	for i := 0; i < 8; i++ {
		var ref struct {
			Ref string `json:"$ref"`
		}
		if json.Unmarshal(raw, &ref) != nil || !strings.HasPrefix(ref.Ref, "#/components/") {
			return raw
		}
		parts := strings.Split(strings.TrimPrefix(ref.Ref, "#/components/"), "/")
		if len(parts) != 2 {
			return raw
		}
		var components map[string]map[string]json.RawMessage
		if json.Unmarshal(spec.Components, &components) != nil {
			return raw
		}
		name := strings.NewReplacer("~1", "/", "~0", "~").Replace(parts[1])
		next, ok := components[parts[0]][name]
		if !ok {
			return raw
		}
		raw = next
	}
	return nil
}
