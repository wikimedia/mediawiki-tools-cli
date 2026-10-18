package devenv

import (
	"io/fs"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/mount"
)

func TestExistingEnvironmentIdentity(t *testing.T) {
	old := Context
	t.Cleanup(func() { Context = old })
	Context = "existing"
	m := DefaultForUser()
	if m.BaseDir != cli.UserDirectoryPathForCmd("mwdd") || m.DockerCompose().Name != "mwcli-mwdd-existing" {
		t.Fatal("the rename must reuse existing contexts and Docker resources")
	}
}

func TestInstalledWikiSettingsCompatibility(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := fs.ReadFile(mount.DevContent, "dev/compose/"+name)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	shim := read("mediawiki/mount/mediawiki/MwddSettings.php")
	if !strings.Contains(shim, "require_once __DIR__ . '/DevSettings.php'") {
		t.Fatal("existing LocalSettings.php must load the current settings")
	}
	for _, service := range []string{"mediawiki", "mediawiki-fresh", "mediawiki-jobrunner", "mediawiki-quibble"} {
		compose := read(service + "/compose.yml")
		for _, path := range []string{"/wm-dev:ro", "/mwdd:ro"} {
			if !strings.Contains(compose, "./compose/mediawiki/mount/mediawiki:"+path) {
				t.Fatalf("%s lacks the settings mount %s", service, path)
			}
		}
	}
}
