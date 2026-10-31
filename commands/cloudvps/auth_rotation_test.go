package cloudvps

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestCloudProfileCredentialRotation(t *testing.T) {
	for _, mode := range []string{"replace", "missing flag", "wrong token project", "different profile project", "invalid credential", "missing profile"} {
		t.Run(mode, func(t *testing.T) {
			isolateCloudProfiles(t)
			old := config.Credential{Key: "cloud_vps.profiles.fixture.secret", Service: "cloudvps", Endpoint: authUrl, Account: "demo:old-id"}
			metadata := map[string]string{"cloud_vps.profiles.fixture.project": "demo", "cloud_vps.profiles.fixture.id": "old-id", "cloud_vps.default_profile": "fixture"}
			if mode != "missing profile" {
				if err := old.SaveWithProvider("old-private-fixture", false, metadata, "keyring"); err != nil {
					t.Fatal(err)
				}
			}
			before := config.PrettyPrint(config.State().OnDiskKoanf)
			previous := loginCloudClient
			t.Cleanup(func() { loginCloudClient = previous })
			project := "demo"
			if mode == "different profile project" {
				project = "another"
			}
			loginCloudClient = func(_ context.Context, auth gophercloud.AuthOptions) (*gophercloud.ProviderClient, error) {
				if auth.ApplicationCredentialID != "new-id" || auth.ApplicationCredentialSecret != "new-private-fixture" {
					t.Fatal("wrong replacement validated")
				}
				if mode == "invalid credential" {
					return nil, errors.New("fixture: invalid application credential")
				}
				authenticatedProject := project
				if mode == "wrong token project" {
					authenticatedProject = "wrong"
				}
				var result tokens.CreateResult
				result.Body = map[string]any{"token": map[string]any{"project": map[string]any{"id": "project-id", "name": authenticatedProject}, "user": map[string]any{"name": "Fixture"}}}
				client := &gophercloud.ProviderClient{}
				if err := client.SetTokenAndAuthResult(result); err != nil {
					t.Fatal(err)
				}
				return client, nil
			}
			cmd := NewCloudVPSCmd()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetIn(strings.NewReader("new-private-fixture\n"))
			args := []string{"auth", "login", "--profile", "fixture", "--project", project, "--id", "new-id", "--secret-stdin"}
			if mode != "missing flag" {
				args = append(args, "--replace")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			if mode == "replace" {
				if err != nil {
					t.Fatal(err)
				}
				if config.State().OnDiskKoanf.String("cloud_vps.profiles.fixture.id") != "new-id" || cloudProfiles.Default() != "fixture" {
					t.Fatal("replacement lost profile or default")
				}
				current := old
				current.Account = "demo:new-id"
				if got, err := current.Read(); err != nil || got != "new-private-fixture" {
					t.Fatal("replacement not saved", err)
				}
				if _, err := old.Read(); err == nil {
					t.Fatal("old identity still accepted")
				}
			} else {
				if err == nil {
					t.Fatal("unsafe rotation accepted")
				}
				if mode == "missing flag" && !strings.Contains(err.Error(), "--replace") {
					t.Fatal("missing rotation instructions", err)
				}
				if config.PrettyPrint(config.State().OnDiskKoanf) != before {
					t.Fatal("failed validation changed configuration")
				}
				if mode != "missing profile" {
					if got, err := old.Read(); err != nil || got != "old-private-fixture" {
						t.Fatal("failed rotation lost old credential", err)
					}
				}
			}
			if strings.Contains(output.String(), "new-private-fixture") || strings.Contains(output.String(), "old-private-fixture") {
				t.Fatal("rotation exposed credentials")
			}
		})
	}
}
