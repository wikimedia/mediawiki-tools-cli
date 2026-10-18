package cloudvps

import (
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestProjectCredentialsResolveKeyringAndBindCredentialID(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	config.State()
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	credential := config.Credential{Key: "cloud_vps.projects.fixture.credential.secret", Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: "fixture:credential-id"}
	if err := credential.SaveWithMetadata("fixture-secret", false, map[string]string{"cloud_vps.projects.fixture.credential.id": "credential-id"}); err != nil {
		t.Fatal(err)
	}
	projects := config.State().Effective.CloudVPS.Projects
	if projects["fixture"].Credential.Secret != "" {
		t.Fatal("secret remains in ordinary configuration")
	}
	got, err := projectCredentials("fixture", projects)
	if err != nil || got.ID != "credential-id" || got.Secret != "fixture-secret" {
		t.Fatalf("cannot resolve stored credential: %v", err)
	}
	projects["fixture"] = config.CloudVPSProject{Credential: config.CloudVPSProjectCredential{ID: "another-id"}}
	if _, err := projectCredentials("fixture", projects); err == nil {
		t.Fatal("reused secret with a different application credential ID")
	}
}
