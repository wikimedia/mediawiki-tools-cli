package cloudvps

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
	"github.com/knadh/koanf"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func isolateCloudProfiles(t *testing.T) {
	t.Helper()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	previous := cli.Opts
	cli.Opts.NoInteraction = true
	keyring.MockInit()
	t.Cleanup(func() { cli.Opts = previous; config.ReApplyKoanfConf(koanf.New(".")) })
}

func TestCloudProfileSelection(t *testing.T) {
	isolateCloudProfiles(t)
	disk := koanf.New(".")
	for name, project := range map[string]string{"reader": "demo", "writer": "demo", "other": "second"} {
		_ = disk.Set("cloud_vps.profiles."+name+".project", project)
		_ = disk.Set("cloud_vps.profiles."+name+".id", name+"-id")
	}
	_ = disk.Set("cloud_vps.default_profile", "reader")
	if err := config.PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	// Selection and listing must work even when the password manager is locked.
	keyring.MockInitWithError(errors.New("locked"))
	for _, tc := range []struct{ profile, project, want, fail string }{
		{"", "", "reader", ""},
		{"", "demo", "reader", ""},
		{"", "second", "other", ""},
		{"writer", "demo", "writer", ""},
		{"writer", "", "writer", ""},
		{"reader", "second", "", "belongs to project"},
		{"absent", "", "", "no saved"},
		{"", "missing", "", "auth login"},
	} {
		t.Run(tc.profile+"/"+tc.project, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().String("profile", tc.profile, "")
			cmd.Flags().String("project", tc.project, "")
			name, _, err := selectedCloudProfile(cmd)
			if tc.fail != "" {
				if err == nil || !strings.Contains(err.Error(), tc.fail) {
					t.Fatalf("unexpected selection failure: %v", err)
				}
				return
			}
			if err != nil || name != tc.want {
				t.Fatalf("selected %q: %v", name, err)
			}
		})
	}
	if err := config.DeleteKeyValueFromDisk("cloud_vps.default_profile"); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("project", "demo", "")
	if _, _, err := selectedCloudProfile(cmd); err == nil || !strings.Contains(err.Error(), "multiple profiles") {
		t.Fatal("ambiguous access level chosen", err)
	}
	if err := config.PutKeyValueOnDisk("cloud_vps.default_project", "second"); err != nil {
		t.Fatal(err)
	}
	cmd = &cobra.Command{}
	if name, project, err := selectedCloudProfile(cmd); err != nil || name != "other" || project != "second" {
		t.Fatalf("default project not respected: %s %s %v", name, project, err)
	}
	cmd.Flags().String("profile", "writer", "")
	if name, project, err := selectedCloudProfile(cmd); err != nil || name != "writer" || project != "demo" {
		t.Fatal("explicit profile did not override default project", err)
	}
	if project, err := projectName(cmd); err != nil || project != "demo" {
		t.Fatal("resource project differs from selected profile", err)
	}
	cmd.Flags().String("project", "second", "")
	if _, err := authenticatedProvider(cmd); err == nil || !strings.Contains(err.Error(), "belongs to project") {
		t.Fatal("mismatch reached the locked credential store", err)
	}
	out, err := executeTestCommand(NewCloudVPSCmd(), "auth", "profiles")
	if err != nil || !strings.Contains(out, "project: demo") || !strings.Contains(out, "writer") {
		t.Fatal("metadata listing failed", err, out)
	}
	matches, _ := cloudProfiles.Complete(nil, nil, "wr")
	if len(matches) != 1 || matches[0] != "writer" {
		t.Fatal("profile completion missing", matches)
	}
}

func TestCloudProfileLoginDefaultsAndRemoval(t *testing.T) {
	isolateCloudProfiles(t)
	previous := loginCloudClient
	t.Cleanup(func() { loginCloudClient = previous })
	authenticatedProject := "demo"
	calls := 0
	loginCloudClient = func(_ context.Context, auth gophercloud.AuthOptions) (*gophercloud.ProviderClient, error) {
		calls++
		if auth.ApplicationCredentialSecret != "private-fixture" {
			t.Fatal("secret not sent for validation")
		}
		var result tokens.CreateResult
		result.Body = map[string]any{"token": map[string]any{"project": map[string]any{"id": "project-id", "name": authenticatedProject}, "user": map[string]any{"name": "Addshore"}}}
		provider := &gophercloud.ProviderClient{}
		if err := provider.SetTokenAndAuthResult(result); err != nil {
			t.Fatal(err)
		}
		return provider, nil
	}
	login := func(name string, extra ...string) (string, error) {
		args := []string{"auth", "login", "--project", "demo", "--id", name + "-id", "--secret", "private-fixture", "--provider", "keyring"}
		if name != "demo" {
			args = append(args, "--profile", name)
		}
		return executeTestCommand(NewCloudVPSCmd(), append(args, extra...)...)
	}
	if out, err := login("demo"); err != nil || !strings.Contains(out, "Default profile: demo") {
		t.Fatal("first profile not default", err, out)
	}
	if _, err := login("readonly"); err != nil {
		t.Fatal(err)
	}
	if cloudProfiles.Default() != "demo" {
		t.Fatal("subsequent login silently switched default")
	}
	if _, err := login("writer", "--default"); err != nil {
		t.Fatal(err)
	}
	if cloudProfiles.Default() != "writer" {
		t.Fatal("explicit default ignored")
	}
	if _, err := executeTestCommand(NewCloudVPSCmd(), "--profile", "readonly", "auth", "default"); err != nil {
		t.Fatal(err)
	}
	if cloudProfiles.Default() != "readonly" {
		t.Fatal("shared default command failed")
	}
	got, err := profileCredentials("readonly")
	if err != nil || got.ID != "readonly-id" || got.Secret != "private-fixture" {
		t.Fatal("credential unavailable", err)
	}
	entries := config.SecretInventory()
	if len(entries) != 3 {
		t.Fatalf("inventory has %d entries, want 3", len(entries))
	}
	for _, entry := range entries {
		if entry.Service != "cloudvps" || entry.Provider != "keyring" || entry.PlaintextPresent {
			t.Fatal("incorrect inventory", entry)
		}
	}
	if _, err := executeTestCommand(NewCloudVPSCmd(), "auth", "remove", "--profile", "readonly"); err != nil {
		t.Fatal(err)
	}
	if cloudProfiles.Default() != "" || config.State().OnDiskKoanf.Exists("cloud_vps.profiles.readonly") {
		t.Fatal("removed profile or default retained")
	}
	if _, err := profileCredentials("writer"); err != nil {
		t.Fatal("removed another profile's credential", err)
	}
	before := config.State().OnDiskKoanf.Raw()
	authenticatedProject = "wrong-project"
	if _, err := login("invalid-project"); err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatal("wrong project credential saved", err)
	}
	if !reflect.DeepEqual(before, config.State().OnDiskKoanf.Raw()) {
		t.Fatal("failed validation changed profiles")
	}
	count := calls
	if _, err := login("bad.profile"); err == nil || calls != count {
		t.Fatal("invalid profile reached authentication", err)
	}
}
