package cobrautil

import "github.com/spf13/cobra"

// ServiceInfo describes the service exposed by a command. Documentation reads
// this metadata from the command tree instead of maintaining a second catalog.
type ServiceInfo struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
	// Logo is an HTTPS image URL, downloaded and cached by the docs build.
	Logo string `json:"logo,omitempty"`
}

// WithServiceInfo attaches service metadata without replacing other annotations.
func WithServiceInfo(command *cobra.Command, info ServiceInfo) *cobra.Command {
	if command.Annotations == nil {
		command.Annotations = make(map[string]string)
	}
	command.Annotations["service-name"] = info.Name
	command.Annotations["service-url"] = info.URL
	command.Annotations["service-logo"] = info.Logo
	return command
}

// CommandServiceInfo returns service metadata, using the command name as a fallback.
func CommandServiceInfo(command *cobra.Command) ServiceInfo {
	name := command.Annotations["service-name"]
	if name == "" {
		name = command.Name()
	}
	return ServiceInfo{Name: name, URL: command.Annotations["service-url"], Logo: command.Annotations["service-logo"]}
}
