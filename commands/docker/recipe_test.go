package docker

import (
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd/recipe"
)

func TestExpandRecipeTemplateVars_ReplacesPortAcrossRelevantSections(t *testing.T) {
	spec := recipe.Spec{
		Name: "wikibase-repoclient",
		LocalSettings: recipe.LocalSettings{
			Files: recipe.LocalSettingsFiles{
				Shared: []recipe.LocalSettingsFile{
					{Content: "$wgWBClientSettings['repoUrl'] = 'http://default.mediawiki.local.wmftest.net:${PORT}';"},
				},
			},
		},
		Maintenance: []recipe.ContainerCommandStep{
			{
				Name:    "add-site",
				Command: []string{"php", "maintenance/run.php", "addSite.php", "--pagepath", "http://default.mediawiki.local.wmftest.net:${PORT}/w/index.php?title=$1"},
			},
		},
	}

	expanded := expandRecipeTemplateVars(spec, map[string]string{"PORT": "19090"})

	shared := expanded.LocalSettings.Files.Shared[0].Content
	if shared != "$wgWBClientSettings['repoUrl'] = 'http://default.mediawiki.local.wmftest.net:19090';" {
		t.Fatalf("unexpected expanded shared content: %q", shared)
	}

	cmdArg := expanded.Maintenance[0].Command[4]
	if cmdArg != "http://default.mediawiki.local.wmftest.net:19090/w/index.php?title=$1" {
		t.Fatalf("unexpected expanded maintenance arg: %q", cmdArg)
	}
}

func TestExpandRecipeEnvVars_UnknownVariableIsPreserved(t *testing.T) {
	got := expandRecipeEnvVars("http://example:${UNKNOWN}/x", map[string]string{"PORT": "1234"})
	want := "http://example:${UNKNOWN}/x"
	if got != want {
		t.Fatalf("expandRecipeEnvVars() = %q, want %q", got, want)
	}
}
