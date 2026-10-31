package cloudvps

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

var cloudProfiles = cloudProfileBackend{Manager: authprofile.Manager{Service: "cloud_vps", Label: "Cloud VPS", SecretField: "secret", Endpoint: secrets.CloudEndpoint}}

type cloudProfileBackend struct{ authprofile.Manager }

func (m cloudProfileBackend) Profiles() ([]authprofile.Profile, error) {
	var profiles []authprofile.Profile
	state := config.State().EffectiveKoanf
	for _, name := range m.Names() {
		prefix := m.Prefix(name) + "."
		profiles = append(profiles, authprofile.Profile{Name: name, Username: state.String(prefix + "id"), Endpoint: secrets.CloudEndpoint, Details: "project: " + state.String(prefix+"project"), Default: name == m.Default()})
	}
	return profiles, nil
}

// An explicitly selected profile must match the selected project. Without one,
// prefer the global default when it matches, otherwise require a unique match.
func selectedCloudProfile(cmd *cobra.Command) (string, string, error) {
	selected, _ := cmd.Flags().GetString("profile")
	project, _ := cmd.Flags().GetString("project")
	return resolveCloudProfile(selected, project)
}

func resolveCloudProfile(selected, project string) (string, string, error) {
	selected = strings.TrimSpace(selected)
	state := config.State().EffectiveKoanf
	if selected != "" {
		name, err := cloudProfiles.Selected(selected)
		if err != nil {
			return "", "", err
		}
		actual := state.String(cloudProfiles.Prefix(name) + ".project")
		if actual == "" {
			return "", "", fmt.Errorf("Cloud VPS profile %q has no project; use auth login", name)
		}
		if project != "" && project != actual {
			return "", "", fmt.Errorf("Cloud VPS profile %q belongs to project %q, not %q", name, actual, project)
		}
		return name, actual, nil
	}
	if project == "" {
		project = state.String("cloud_vps.default_project")
	}
	if name := cloudProfiles.Default(); name != "" {
		actual := state.String(cloudProfiles.Prefix(name) + ".project")
		if actual != "" && (project == "" || project == actual) {
			return name, actual, nil
		}
	}
	if project == "" {
		return "", "", fmt.Errorf("select a Cloud VPS profile with --profile, or a project with --project")
	}
	var matches []string
	for _, name := range cloudProfiles.Names() {
		if state.String(cloudProfiles.Prefix(name)+".project") == project {
			matches = append(matches, name)
		}
	}
	if len(matches) == 1 {
		return matches[0], project, nil
	}
	if len(matches) > 1 {
		return "", "", fmt.Errorf("multiple profiles for project %q; select one with --profile: %s", project, strings.Join(matches, ", "))
	}
	return "", "", fmt.Errorf("no application credentials configured for project %q; use cloudvps auth login --project %s", project, project)
}

func profileCredentials(name string) (config.CloudVPSProjectCredential, error) {
	prefix := cloudProfiles.Prefix(name) + "."
	state := config.State().EffectiveKoanf
	id := state.String(prefix + "id")
	if id == "" {
		return config.CloudVPSProjectCredential{}, fmt.Errorf("application credential ID is missing for Cloud VPS profile %q", name)
	}
	credential := config.Credential{Key: prefix + "secret", Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: state.String(prefix+"project") + ":" + id}
	secret, err := credential.Read()
	if err != nil {
		return config.CloudVPSProjectCredential{}, err
	}
	if secret == "" {
		return config.CloudVPSProjectCredential{}, fmt.Errorf("no secret saved for Cloud VPS profile %q; use auth login", name)
	}
	return config.CloudVPSProjectCredential{ID: id, Secret: secret}, nil
}
