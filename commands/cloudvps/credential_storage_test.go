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
	credential := config.Credential{Key: "cloud_vps.profiles.fixture.secret", Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: "fixture:credential-id"}
	if err := credential.SaveWithMetadata("fixture-secret", false, map[string]string{"cloud_vps.profiles.fixture.id": "credential-id", "cloud_vps.profiles.fixture.project": "fixture"}); err != nil {
		t.Fatal(err)
	}
	if config.State().EffectiveKoanf.String("cloud_vps.profiles.fixture.secret") != "" {
		t.Fatal("secret remains in ordinary configuration")
	}
	got, err := profileCredentials("fixture")
	if err != nil || got.ID != "credential-id" || got.Secret != "fixture-secret" {
		t.Fatalf("cannot resolve stored credential: %v", err)
	}
	if err := config.PutKeyValueOnDisk("cloud_vps.profiles.fixture.secret", "leftover-plaintext"); err != nil {
		t.Fatal(err)
	}
	got, err = profileCredentials("fixture")
	if err != nil || got.Secret != "fixture-secret" {
		t.Fatal("leftover plaintext bypassed the saved provider")
	}
	if err := config.PutKeyValueOnDisk("cloud_vps.profiles.fixture.id", "another-id"); err != nil {
		t.Fatal(err)
	}
	if _, err := profileCredentials("fixture"); err == nil {
		t.Fatal("reused secret with a different application credential ID")
	}
}
