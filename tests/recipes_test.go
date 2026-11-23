package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv/recipe"
)

// TestEmbeddedRecipes runs sequentially: recipes can publish fixed host ports.
// Enable only on a disposable Docker host, as with the other Docker tests.
func TestEmbeddedRecipes(t *testing.T) {
	if os.Getenv("MWCLI_TEST_RECIPES") != "1" {
		t.Skip("run via tests/test-docker-recipes.sh on a disposable Docker host")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("MW_BIN")
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(root, binary)
	}
	paths, err := filepath.Glob(filepath.Join(root, "mount/dev/recipes/*.yml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("discover embedded recipes: %v (%d found)", err, len(paths))
	}
	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".yml"), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			spec, err := recipe.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			work := t.TempDir()
			name := strings.TrimSuffix(filepath.Base(path), ".yml")
			context := fmt.Sprintf("recipes-%d-%s", os.Getpid(), name)
			checkout := filepath.Join(work, "mediawiki")
			run := func(name string, args ...string) error {
				t.Helper()
				t.Logf("%s %s", name, strings.Join(args, " "))
				cmd := osexec.Command(name, args...)
				cmd.Dir = root
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				return cmd.Run()
			}
			devArgs := []string{"--no-interaction", "dev", "--context", context}
			dev := func(args ...string) error {
				return run(binary, append(append([]string{}, devArgs...), args...)...)
			}
			hosts := func(args ...string) error {
				if osexec.Command(binary, append(devArgs, "hosts", "writable")...).Run() == nil {
					return dev(append([]string{"hosts"}, args...)...)
				}
				command := append([]string{"-E", binary}, devArgs...)
				command = append(command, "hosts")
				return run("sudo", append(command, args...)...)
			}
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(run("cp", "-a", "--reflink=auto", filepath.Join(root, ".mediawiki"), checkout))
			t.Cleanup(func() {
				if t.Failed() {
					_ = dev("compose", "logs", "--", "--tail", "100")
				}
				// Remove hosts before destroy clears the context's used-host record.
				if err := hosts("remove"); err != nil {
					t.Error(err)
				}
				if err := dev("destroy"); err != nil {
					t.Error(err)
				}
				if err := dev("env", "clear"); err != nil {
					t.Error(err)
				}
			})
			must(dev("env", "set", "MEDIAWIKI_VOLUMES_CODE", checkout))
			hostArgs := []string{"add"}
			for _, site := range spec.Sites {
				hostArgs = append(hostArgs, "--host", site.DBName+".mediawiki.local.wmftest.net")
			}
			must(hosts(hostArgs...))
			must(dev("recipe", "validate", "--name", name))
			must(dev("recipe", "--name", name))
			port, err := osexec.Command(binary, append(devArgs, "env", "get", "PORT")...).Output()
			must(err)
			client := &http.Client{Timeout: 10 * time.Second}
			for _, site := range spec.Sites {
				t.Run(site.DBName, func(t *testing.T) {
					url := fmt.Sprintf("http://127.0.0.1:%s/w/api.php?action=query&meta=siteinfo&format=json", strings.TrimSpace(string(port)))
					request, err := http.NewRequest(http.MethodGet, url, nil)
					if err != nil {
						t.Fatal(err)
					}
					request.Host = site.DBName + ".mediawiki.local.wmftest.net"
					privateWiki := name == "wmf-wikifarm" && site.DBName == "officewiki"
					var last string
					for attempt := 0; attempt < 30; attempt++ {
						response, err := client.Do(request)
						if err == nil {
							var result struct {
								Query struct {
									General struct {
										WikiID string `json:"wikiid"`
									} `json:"general"`
								} `json:"query"`
								Error struct{ Code string } `json:"error"`
							}
							err = json.NewDecoder(response.Body).Decode(&result)
							response.Body.Close()
							if response.StatusCode == http.StatusOK && err == nil &&
								(result.Query.General.WikiID == site.DBName || (privateWiki && result.Error.Code == "readapidenied")) {
								return
							}
							last = fmt.Sprintf("HTTP %d, API: %+v, decode: %v", response.StatusCode, result, err)
						} else {
							last = err.Error()
						}
						time.Sleep(2 * time.Second)
					}
					t.Fatalf("wiki API did not become ready: %s", last)
				})
			}
			if name == "wmf-wikifarm" {
				cmd := osexec.Command("bash", "tests/test-docker-recipe-wmf-wikifarm.sh")
				cmd.Dir = root
				cmd.Env = append(os.Environ(), "CONTEXT="+context, "MWCLI_BIN="+binary)
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				must(cmd.Run())
			}
		})
	}
}
