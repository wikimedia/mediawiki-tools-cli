package recipe

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	TypeMWCLIDevRecipe = "mwcli.dev/recipe"
	TypeMWCLIRecipe    = "mwcli/recipe"
)

type Spec struct {
	Type          string                 `yaml:"type"`
	Version       string                 `yaml:"version"`
	Name          string                 `yaml:"name"`
	Description   string                 `yaml:"description"`
	Source        Source                 `yaml:"source"`
	Env           map[string]string      `yaml:"env"`
	Code          Code                   `yaml:"code"`
	Services      []Service              `yaml:"services"`
	Sites         []Site                 `yaml:"sites"`
	JobRunner     JobRunner              `yaml:"jobRunner"`
	LocalSettings LocalSettings          `yaml:"localSettings"`
	Content       Content                `yaml:"content"`
	Maintenance   []ContainerCommandStep `yaml:"maintenance"`
	Patches       []Patch                `yaml:"patches"`
	CustomCompose CustomCompose          `yaml:"customCompose"`
}

type Source struct {
	UseGithub             bool   `yaml:"useGithub"`
	Shallow               bool   `yaml:"shallow"`
	GerritInteractionType string `yaml:"gerritInteractionType"`
	GerritUsername        string `yaml:"gerritUsername"`
}

type Code struct {
	Core       bool       `yaml:"core"`
	Extensions []Checkout `yaml:"extensions"`
	Skins      []Checkout `yaml:"skins"`
}

// Checkout represents a code checkout. Either Name (a Gerrit extension/skin
// short name) OR URL+Path (an arbitrary git repository) must be provided.
type Checkout struct {
	Name string `yaml:"name"` // Gerrit short name, e.g. "VisualEditor"
	URL  string `yaml:"url"`  // Arbitrary git clone URL
	Path string `yaml:"path"` // Destination path relative to MediaWiki root
}

type Service struct {
	Name  string `yaml:"name"`
	State string `yaml:"state"` // started (default), stopped
}

type Site struct {
	DBName string `yaml:"dbname"`
	DBType string `yaml:"dbtype"`
}

type JobRunner struct {
	Sites []string `yaml:"sites"`
}

type LocalSettings struct {
	AppendPHP        string             `yaml:"appendPHP"`
	Files            LocalSettingsFiles `yaml:"files"`
	YAMLSettingsFile string             `yaml:"yamlSettingsFile"`
	YAMLSettings     string             `yaml:"yamlSettings"`
}

type LocalSettingsFiles struct {
	Shared  []LocalSettingsFile            `yaml:"shared"`
	PerWiki map[string][]LocalSettingsFile `yaml:"perWiki"`
}

type LocalSettingsFile struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

type Content struct {
	Wikibase WikibaseContent `yaml:"wikibase"`
	Pages    []PageContent   `yaml:"pages"`
}

type WikibaseContent struct {
	Properties []WikibaseProperty `yaml:"properties"`
	Items      []WikibaseItem     `yaml:"items"`
}

type WikibaseProperty struct {
	ID       string `yaml:"id"`
	Label    string `yaml:"label"`
	Datatype string `yaml:"datatype"`
}

type WikibaseItem struct {
	ID     string              `yaml:"id"`
	Label  string              `yaml:"label"`
	Claims []WikibaseItemClaim `yaml:"claims"`
}

type WikibaseItemClaim struct {
	Property string `yaml:"property"`
	Value    string `yaml:"value"`
}

type PageContent struct {
	Wiki  string `yaml:"wiki"`
	Title string `yaml:"title"`
	Text  string `yaml:"text"`
}

type ContainerCommandStep struct {
	Name       string            `yaml:"name"`
	Command    []string          `yaml:"command"`
	User       string            `yaml:"user"`
	WorkingDir string            `yaml:"workingDir"`
	Env        map[string]string `yaml:"env"`
}

type Patch struct {
	Name       string   `yaml:"name"`
	RepoPath   string   `yaml:"repoPath"`
	Fetch      []string `yaml:"fetch"`
	CherryPick string   `yaml:"cherryPick"` // defaults to FETCH_HEAD
}

type CustomCompose struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

func Parse(content []byte) (Spec, error) {
	var spec Spec
	if err := yaml.Unmarshal(content, &spec); err != nil {
		return Spec{}, err
	}
	spec.Normalize()
	if err := spec.Validate(); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

func (s *Spec) Normalize() {
	s.Type = strings.TrimSpace(s.Type)
	s.Version = strings.TrimSpace(s.Version)
	s.Name = strings.TrimSpace(s.Name)
	if s.Env == nil {
		s.Env = map[string]string{}
	}

	if s.CustomCompose.Name == "" && strings.TrimSpace(s.CustomCompose.Content) != "" {
		s.CustomCompose.Name = "custom"
	}

	for i := range s.Services {
		s.Services[i].Name = strings.TrimSpace(s.Services[i].Name)
		s.Services[i].State = strings.TrimSpace(strings.ToLower(s.Services[i].State))
		if s.Services[i].State == "" {
			s.Services[i].State = "started"
		}
	}

	for i := range s.Sites {
		s.Sites[i].DBName = strings.TrimSpace(s.Sites[i].DBName)
		s.Sites[i].DBType = strings.TrimSpace(strings.ToLower(s.Sites[i].DBType))
		if s.Sites[i].DBType == "" {
			s.Sites[i].DBType = "mysql"
		}
	}

	s.JobRunner.Sites = normalizeStrings(s.JobRunner.Sites)

	s.Code.Extensions = normalizeCheckouts(s.Code.Extensions)
	s.Code.Skins = normalizeCheckouts(s.Code.Skins)
}

func normalizeStrings(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func normalizeCheckouts(in []Checkout) []Checkout {
	out := make([]Checkout, 0, len(in))
	seenNames := map[string]bool{}
	seenURLPath := map[string]bool{}
	for _, c := range in {
		name := strings.TrimSpace(c.Name)
		url := strings.TrimSpace(c.URL)
		path := strings.TrimSpace(c.Path)
		if name != "" {
			if !seenNames[name] {
				seenNames[name] = true
				out = append(out, Checkout{Name: name})
			}
		} else if url != "" {
			key := url + "\x00" + path
			if !seenURLPath[key] {
				seenURLPath[key] = true
				out = append(out, Checkout{URL: url, Path: path})
			}
		}
	}
	return out
}

func (s Spec) Validate() error {
	if s.Type == "" {
		return fmt.Errorf("recipe type is required")
	}
	if s.Type != TypeMWCLIDevRecipe && s.Type != TypeMWCLIRecipe {
		return fmt.Errorf("unsupported recipe type %q", s.Type)
	}
	if s.Version == "" {
		return fmt.Errorf("recipe version is required")
	}

	if len(s.Services) == 0 && len(s.Sites) == 0 && len(s.JobRunner.Sites) == 0 && !s.Code.Core && len(s.Code.Extensions) == 0 && len(s.Code.Skins) == 0 && strings.TrimSpace(s.LocalSettings.AppendPHP) == "" && len(s.LocalSettings.Files.Shared) == 0 && len(s.LocalSettings.Files.PerWiki) == 0 && len(s.Content.Wikibase.Properties) == 0 && len(s.Content.Wikibase.Items) == 0 && len(s.Content.Pages) == 0 && len(s.Maintenance) == 0 {
		return fmt.Errorf("recipe is empty: define at least one setup operation")
	}

	for _, service := range s.Services {
		if service.Name == "" {
			return fmt.Errorf("service name cannot be empty")
		}
		if service.State != "started" && service.State != "stopped" {
			return fmt.Errorf("service %q has unsupported state %q", service.Name, service.State)
		}
	}

	for _, co := range append(s.Code.Extensions, s.Code.Skins...) {
		if co.Name == "" && co.URL == "" {
			return fmt.Errorf("checkout entry has neither name nor url")
		}
		if co.URL != "" && co.Path == "" {
			return fmt.Errorf("checkout with url %q requires a path", co.URL)
		}
	}

	for _, site := range s.Sites {
		if site.DBName == "" {
			return fmt.Errorf("site dbname cannot be empty")
		}
		switch site.DBType {
		case "mysql", "postgres", "sqlite":
		default:
			return fmt.Errorf("site %q has unsupported dbtype %q", site.DBName, site.DBType)
		}
	}

	for _, site := range s.JobRunner.Sites {
		if strings.TrimSpace(site) == "" {
			return fmt.Errorf("jobRunner site cannot be empty")
		}
	}

	for _, step := range s.Maintenance {
		if len(step.Command) == 0 {
			return fmt.Errorf("maintenance step %q has no command", step.Name)
		}
	}

	for _, patch := range s.Patches {
		if patch.RepoPath == "" {
			return fmt.Errorf("patch %q has empty repoPath", patch.Name)
		}
		if len(patch.Fetch) == 0 {
			return fmt.Errorf("patch %q has empty fetch", patch.Name)
		}
	}

	if s.LocalSettings.YAMLSettingsFile != "" && strings.TrimSpace(s.LocalSettings.YAMLSettings) == "" {
		return fmt.Errorf("localSettings.yamlSettingsFile was provided without localSettings.yamlSettings content")
	}

	// Validate content
	for _, prop := range s.Content.Wikibase.Properties {
		if prop.ID == "" {
			return fmt.Errorf("wikibase property has empty id")
		}
		if prop.Label == "" {
			return fmt.Errorf("wikibase property %q has empty label", prop.ID)
		}
		if prop.Datatype == "" {
			return fmt.Errorf("wikibase property %q has empty datatype", prop.ID)
		}
	}

	for _, item := range s.Content.Wikibase.Items {
		if item.ID == "" {
			return fmt.Errorf("wikibase item has empty id")
		}
		if item.Label == "" {
			return fmt.Errorf("wikibase item %q has empty label", item.ID)
		}
		for _, claim := range item.Claims {
			if claim.Property == "" {
				return fmt.Errorf("wikibase item %q has claim with empty property", item.ID)
			}
		}
	}

	for _, page := range s.Content.Pages {
		if page.Wiki == "" {
			return fmt.Errorf("page has empty wiki")
		}
		if page.Title == "" {
			return fmt.Errorf("page on wiki %q has empty title", page.Wiki)
		}
	}

	return nil
}
