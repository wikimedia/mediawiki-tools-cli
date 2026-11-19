package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

const manualMarker = "<!-- Generated from command help; edit the command source. -->"

type manualSidebarItem struct {
	Label     string              `json:"label"`
	Link      string              `json:"link,omitempty"`
	Collapsed bool                `json:"collapsed,omitempty"`
	Items     []manualSidebarItem `json:"items,omitempty"`
}

type manualCommandReference struct {
	Name    string   `json:"name"`
	Title   string   `json:"title"`
	Link    string   `json:"link"`
	Parent  string   `json:"parent,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

type manualMetadata struct {
	Services           []manualService          `json:"services"`
	Commands           []manualCommandReference `json:"commands"`
	Catalog            []manualCatalogEntry     `json:"catalog"`
	Version            documentationVersion     `json:"version"`
	Pages              int                      `json:"pages"`
	Sidebar            []manualSidebarItem      `json:"sidebar"`
	DevelopmentSidebar []manualSidebarItem      `json:"developmentSidebar"`
}

type manualCatalogEntry struct {
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Aliases     []string                `json:"aliases,omitempty"`
	Link        string                  `json:"link,omitempty"`
	Hidden      bool                    `json:"hidden,omitempty"`
	Deprecated  bool                    `json:"deprecated,omitempty"`
	Hints       *commandmeta.Hints      `json:"hints,omitempty"`
	HighRisk    bool                    `json:"highRisk,omitempty"`
	MCPTools    []mcpserver.CommandTool `json:"mcpTools,omitempty"`
}

type manualService struct {
	cobrautil.ServiceInfo
	Link        string          `json:"link"`
	Command     string          `json:"command"`
	Development bool            `json:"development"`
	Children    []manualService `json:"children,omitempty"`
}

func generateManual(directory string) error {
	version := currentDocumentationVersion()
	// Construct help with an empty application configuration. Generation must
	// neither read saved credentials nor write to the developer's configuration.
	temporary, err := os.MkdirTemp("", "mwcli-manual-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for key, value := range map[string]string{
		"MWCLI_SKIP_RENDER_MARKDOWN": "true", "MWCLI_CONTEXT_TEST": "1",
		"GITLAB_CI": "true", "CI_PROJECT_DIR": temporary,
		"GLAB_CONFIG_DIR": filepath.Join(temporary, "glab"), "GH_CONFIG_DIR": filepath.Join(temporary, "gh"),
	} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	root := cmd.NewMwCliCmd()
	root.InitDefaultCompletionCmd()
	return writeManual(root, directory, version, cmd.MCPCommandTools()...)
}

func manualURL(command *cobra.Command) string {
	var parts []string
	for current := command; current.HasParent(); current = current.Parent() {
		parts = append([]string{current.Name()}, parts...)
	}
	if len(parts) == 0 {
		return "/manual/"
	}
	return "/manual/" + strings.Join(parts, "/") + "/"
}

func manualFilename(command *cobra.Command) string {
	path := strings.TrimPrefix(manualURL(command), "/manual/")
	if path == "" {
		return "index.md"
	}
	return path + "index.md"
}

func writeManual(root *cobra.Command, directory string, version documentationVersion, tools ...mcpserver.CommandTool) error {
	toolsByCommand := map[string][]mcpserver.CommandTool{}
	for _, tool := range tools {
		if err := tool.ToolAccess.Validate(); err != nil {
			return fmt.Errorf("MCP tool %q: %w", tool.Name, err)
		}
		if tool.Profile != nil {
			if err := tool.Profile.Validate(); err != nil {
				return fmt.Errorf("MCP tool %q with profile: %w", tool.Name, err)
			}
		}
		toolsByCommand[root.Name()+" "+tool.Command] = append(toolsByCommand[root.Name()+" "+tool.Command], tool)
	}
	var catalog []manualCatalogEntry
	var catalogErr error
	var scan func(*cobra.Command, bool, bool)
	scan = func(command *cobra.Command, hidden, deprecated bool) {
		hidden = hidden || command.Hidden
		deprecated = deprecated || command.Deprecated != ""
		if command.Runnable() {
			entry := manualCatalogEntry{
				Title: command.CommandPath(), Description: command.Short, Aliases: command.Aliases,
				Hidden: hidden, Deprecated: deprecated, MCPTools: toolsByCommand[command.CommandPath()],
			}
			delete(toolsByCommand, entry.Title)
			hints, err := commandmeta.Read(command)
			if err == nil {
				entry.Hints = &hints
				entry.HighRisk = hints.AlwaysRequiresApproval()
			} else if !errors.Is(err, commandmeta.ErrMissingHints) && catalogErr == nil {
				catalogErr = fmt.Errorf("%s: %w", entry.Title, err)
			}
			catalog = append(catalog, entry)
		}
		for _, child := range command.Commands() {
			scan(child, hidden, deprecated)
		}
	}
	scan(root, false, false)
	if catalogErr != nil {
		return catalogErr
	}
	for path, tools := range toolsByCommand {
		return fmt.Errorf("MCP tool %q refers to an unknown executable command %q", tools[0].Name, path)
	}
	pruneReferenceChildren(root)
	var commands []*cobra.Command
	var collect func(*cobra.Command)
	collect = func(command *cobra.Command) {
		commands = append(commands, command)
		for _, child := range command.Commands() {
			if manualCommandAvailable(child) {
				collect(child)
			}
		}
	}
	collect(root)
	metadata := manualMetadata{Version: version, Pages: len(commands), Catalog: catalog}
	links := make(map[string]string, len(commands))
	for _, command := range commands {
		command.Example = cobrautil.CommandExamples(command)
		command.DisableAutoGenTag = true
		reference := manualCommandReference{Name: command.Name(), Title: command.CommandPath(), Link: manualURL(command), Aliases: command.Aliases}
		links[reference.Title] = reference.Link
		if command.HasParent() {
			reference.Parent = manualURL(command.Parent())
		}
		metadata.Commands = append(metadata.Commands, reference)
		if command.GroupID == "service" && (command.Parent() == root || reference.Parent == "/manual/dev/") {
			metadata.Services = append(metadata.Services, manualServiceReference(command, command.Parent() != root))
		}
	}
	for i := range metadata.Catalog {
		metadata.Catalog[i].Link = links[metadata.Catalog[i].Title]
	}
	metadata.Sidebar = manualSidebar(root)
	for _, child := range root.Commands() {
		if child.Name() == "dev" && manualCommandAvailable(child) {
			metadata.DevelopmentSidebar = manualSidebar(child)
		}
	}
	planned := map[string][]byte{"catalog/index.md": []byte(`---
title: Command catalog
description: Search every executable command by action, risk, and MCP support.
editUrl: false
---

` + manualMarker + `

Every executable command in this build, including hidden commands. Capability
hints describe what a command can do. **Unclassified** means it has no hints.
For approval and visibility rules, see [Phabricator access](/guides/phabricator/access).
`)}
	for _, command := range commands {
		content := manualContent(command)
		title, _ := json.Marshal(command.CommandPath())
		description, _ := json.Marshal(command.Short)
		label, _ := json.Marshal(version.Label)
		frontmatter := fmt.Sprintf("---\ntitle: %s\ndescription: %s\nmanualVersion: %s\nmanualUnreleased: %t\neditUrl: false\n---\n\n%s\n\n", title, description, label, version.Unreleased, manualMarker)
		planned[manualFilename(command)] = []byte(frontmatter + content)
	}
	for name, data := range planned {
		filename := filepath.Join(directory, filepath.FromSlash(name))
		if previous, err := os.ReadFile(filename); err == nil && !bytes.Contains(previous, []byte(manualMarker)) {
			return fmt.Errorf("refusing to overwrite hand-written manual page %s", filename)
		}
		if err := writeManualFile(filename, data); err != nil {
			return err
		}
	}
	if err := filepath.WalkDir(directory, func(filename string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(filename) != ".md" {
			return err
		}
		relative, err := filepath.Rel(directory, filename)
		if err != nil {
			return err
		}
		if _, found := planned[filepath.ToSlash(relative)]; found {
			return nil
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(manualMarker)) {
			return os.Remove(filename)
		}
		return nil
	}); err != nil {
		return err
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	if err := writeManualFile(filepath.Join(directory, "_meta.json"), append(data, '\n')); err != nil {
		return err
	}
	fmt.Printf("Generated %d manual pages in %s\n", len(commands), directory)
	return nil
}

func manualServiceReference(command *cobra.Command, development bool) manualService {
	service := manualService{
		ServiceInfo: cobrautil.CommandServiceInfo(command), Link: manualURL(command),
		Command: command.CommandPath(), Development: development,
	}
	for _, child := range command.Commands() {
		if manualCommandAvailable(child) && child.Annotations["service-name"] != "" {
			service.Children = append(service.Children, manualServiceReference(child, development))
		}
	}
	return service
}

func manualSidebar(command *cobra.Command) []manualSidebarItem {
	var sidebar []manualSidebarItem
	for _, category := range manualCommandGroups(command) {
		section := manualSidebarItem{Label: category.title}
		for _, child := range category.commands {
			item := manualSidebarItem{Label: child.Name(), Link: manualURL(child)}
			section.Items = append(section.Items, item)
		}
		sidebar = append(sidebar, section)
	}
	return sidebar
}

// Render reference pages for reading on the web rather than copying the
// terminal help layout (fixed-width flag tables and boilerplate headings).
func manualContent(command *cobra.Command) string {
	command.InitDefaultHelpFlag()
	var body strings.Builder
	if command.Runnable() {
		fmt.Fprintf(&body, "```text\n%s\n```\n\n", command.UseLine())
	}
	description := command.Long
	if description == "" {
		description = command.Short
	}
	// Some embedded descriptions include a document title; the page already has one.
	if heading, rest, found := strings.Cut(strings.TrimSpace(description), "\n"); found && strings.HasPrefix(heading, "# ") {
		description = rest
	}
	body.WriteString(strings.TrimSpace(description) + "\n\n")
	for _, category := range manualCommandGroups(command) {
		title := category.title
		if len(command.Groups()) == 0 {
			title = "Commands"
		}
		fmt.Fprintf(&body, "## %s\n\n", title)
		for _, child := range category.commands {
			fmt.Fprintf(&body, "* [%s](%s) — %s\n", child.CommandPath(), manualURL(child), child.Short)
		}
		body.WriteString("\n")
	}
	writeManualOptions(&body, "Options", command.NonInheritedFlags())
	writeManualOptions(&body, "Global options", command.InheritedFlags())
	if command.Example != "" {
		fmt.Fprintf(&body, "## Examples\n\n```sh\n%s\n```\n\n", strings.TrimSpace(dedent.Dedent(command.Example)))
	}
	if command.HasParent() {
		fmt.Fprintf(&body, "## See also\n\n* [%s](%s)\n", command.Parent().CommandPath(), manualURL(command.Parent()))
	}
	return body.String()
}

func writeManualOptions(body *strings.Builder, title string, flags *pflag.FlagSet) {
	if !flags.HasAvailableFlags() {
		return
	}
	fmt.Fprintf(body, "## %s\n\n<dl class=\"command-options\">\n", title)
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || flag.Deprecated != "" {
			return
		}
		name, usage := pflag.UnquoteUsage(flag)
		label := "--" + flag.Name
		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			label = "-" + flag.Shorthand + ", " + label
		}
		if name != "" {
			label += " <" + name + ">"
		}
		fmt.Fprintf(body, "<dt><code>%s</code></dt>\n<dd>%s", html.EscapeString(label), html.EscapeString(usage))
		if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "0" && flag.DefValue != "[]" {
			fmt.Fprintf(body, " <span class=\"option-default\">Default: <code>%s</code>.</span>", html.EscapeString(flag.DefValue))
		}
		body.WriteString("</dd>\n")
	})
	body.WriteString("</dl>\n\n")
}

// Include Cobra's help command, which IsAvailableCommand deliberately excludes
// from generated reference lists even though it is shown in terminal help.
func manualCommandAvailable(command *cobra.Command) bool {
	return !command.Hidden && command.Deprecated == "" && (command.Runnable() || command.HasAvailableSubCommands())
}

type manualCommandGroup struct {
	title    string
	commands []*cobra.Command
}

func manualCommandGroups(command *cobra.Command) []manualCommandGroup {
	var groups []manualCommandGroup
	for _, group := range command.Groups() {
		title := strings.TrimSuffix(group.Title, " Commands")
		if title == "Service" {
			title = "Services"
		} else if title == "Core" {
			title = "Environment"
		}
		category := manualCommandGroup{title: title}
		for _, child := range command.Commands() {
			if child.GroupID == group.ID && manualCommandAvailable(child) {
				category.commands = append(category.commands, child)
			}
		}
		if len(category.commands) > 0 {
			groups = append(groups, category)
		}
	}
	additional := manualCommandGroup{title: "Other"}
	for _, child := range command.Commands() {
		if child.GroupID == "" && manualCommandAvailable(child) {
			additional.commands = append(additional.commands, child)
		}
	}
	if len(additional.commands) > 0 {
		groups = append(groups, additional)
	}
	return groups
}

func writeManualFile(filename string, data []byte) error {
	if previous, err := os.ReadFile(filename); err == nil && bytes.Equal(previous, data) {
		return nil // Preserve mtimes so unchanged pages do not trigger live reload.
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".manual-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), filename)
}

// pruneReferenceChildren applies reference exclusions to a disposable command
// tree. Runtime help and completion keep the complete upstream command tree.
func pruneReferenceChildren(command *cobra.Command) {
	if command.Annotations["mwcli-docs-skip-children"] != "" {
		command.RemoveCommand(command.Commands()...)
		return
	}
	for _, child := range command.Commands() {
		pruneReferenceChildren(child)
	}
}
