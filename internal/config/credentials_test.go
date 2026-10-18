package config

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/knadh/koanf"
)

func isolatedConfig(t *testing.T) {
	t.Helper()
	oldK, oldDisk, oldC, oldCDisk, oldLoaded, oldHome := k, kOnDisk, c, cOnDisk, kLoaded, xdg.ConfigHome
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	xdg.ConfigHome = t.TempDir()
	k, kOnDisk, c, cOnDisk, kLoaded = koanf.New("."), koanf.New("."), Config{}, Config{}, false
	t.Cleanup(func() {
		k, kOnDisk, c, cOnDisk, kLoaded, xdg.ConfigHome = oldK, oldDisk, oldC, oldCDisk, oldLoaded, oldHome
	})
	State()
}

func TestRedactionDoesNotMutateConfig(t *testing.T) {
	isolatedConfig(t)
	disk := State().OnDiskKoanf.Copy()
	for _, key := range []string{"gerrit.password", "cloud_vps.projects.demo.credential.secret", "phabricator.sites.demo.key", "service.api_token"} {
		if err := disk.Set(key, "test-secret"); err != nil {
			t.Fatal(err)
		}
	}
	redacted := Redacted(disk)
	if strings.Contains(PrettyPrint(redacted), "test-secret") {
		t.Fatal("secret visible in redacted output")
	}
	if disk.String("gerrit.password") != "test-secret" {
		t.Fatal("redaction changed stored credential")
	}
}

func TestConfigWriteRepairsPermissionsAndRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions and symlinks")
	}
	isolatedConfig(t)
	if err := os.Chmod(os.Getenv("CI_PROJECT_DIR"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(Path(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateConfig([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(Path())
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode %v", info.Mode())
	}
	dirInfo, _ := os.Stat(os.Getenv("CI_PROJECT_DIR"))
	if dirInfo.Mode().Perm() != 0o755 {
		t.Fatalf("unexpected checkout permissions %v", dirInfo.Mode())
	}
	target := t.TempDir() + "/target"
	if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, Path()); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateConfig([]byte("overwritten")); err == nil {
		t.Fatal("followed config symlink")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "untouched" {
		t.Fatal("overwrote symlink target")
	}
}

func TestUserConfigDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions")
	}
	isolatedConfig(t)
	if err := os.Unsetenv("GITLAB_CI"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(xdg.ConfigHome+"/mwcli", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateConfig([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(xdg.ConfigHome + "/mwcli")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode %v", info.Mode())
	}
}
