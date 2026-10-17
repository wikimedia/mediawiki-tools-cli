package cloudvps

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func testProvider(t *testing.T, handler http.HandlerFunc) providerFactory {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return func(cmd *cobra.Command) (*gophercloud.ProviderClient, error) {
		return &gophercloud.ProviderClient{
			TokenID: "test-token",
			EndpointLocator: func(opts gophercloud.EndpointOpts) (string, error) {
				return server.URL + "/" + opts.Type + "/", nil
			},
		}, nil
	}
}

func executeTestCommand(cmd *cobra.Command, args ...string) (string, error) {
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func TestComputeCreate(t *testing.T) {
	userData := "#cloud-config\nruncmd:\n  - echo hello\n"
	path := filepath.Join(t.TempDir(), "cloud-init.yaml")
	if err := os.WriteFile(path, []byte(userData), 0o600); err != nil {
		t.Fatal(err)
	}
	var requests []string
	provider := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /compute/servers":
			var body struct {
				Server map[string]any `json:"server"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			for key, want := range map[string]any{"name": "ci-runner", "imageRef": "image-id", "flavorRef": "flavor-id", "key_name": "my-key", "user_data": base64.StdEncoding.EncodeToString([]byte(userData))} {
				if body.Server[key] != want {
					t.Errorf("%s: got %v, want %v", key, body.Server[key], want)
				}
			}
			if !reflect.DeepEqual(body.Server["networks"], []any{map[string]any{"uuid": "network-id"}}) {
				t.Errorf("wrong networks: %v", body.Server["networks"])
			}
			if !reflect.DeepEqual(body.Server["security_groups"], []any{map[string]any{"name": "default"}}) {
				t.Errorf("wrong security groups: %v", body.Server["security_groups"])
			}
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"server":{"id":"server-id","name":"ci-runner","adminPass":"do-not-print"}}`)
		case "GET /compute/servers/server-id":
			fmt.Fprint(w, `{"server":{"id":"server-id","status":"ACTIVE"}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	output, err := executeTestCommand(newComputeCreateCmd(provider), "ci-runner", "--image", "image-id", "--flavor", "flavor-id", "--network", "network-id", "--key-name", "my-key", "--user-data", path, "--security-group", "default", "--wait")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "server-id") || strings.Contains(output, "do-not-print") {
		t.Fatalf("unexpected output: %s", output)
	}
	if len(requests) != 2 {
		t.Fatalf("unexpected requests: %v", requests)
	}
}

func TestVolumeCreateAndAttach(t *testing.T) {
	var requests []string
	provider := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /block-storage/volumes":
			var body struct {
				Volume struct {
					Name string `json:"name"`
					Size int    `json:"size"`
				} `json:"volume"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Volume.Name != "ci-docker" || body.Volume.Size != 40 {
				t.Errorf("wrong volume request: %+v", body)
			}
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"volume":{"id":"volume-id","status":"creating"}}`)
		case "GET /block-storage/volumes/volume-id":
			status := "available"
			if len(requests) > 2 {
				status = "in-use"
			}
			fmt.Fprintf(w, `{"volume":{"id":"volume-id","status":%q}}`, status)
		case "POST /compute/servers/server-id/os-volume_attachments":
			var body map[string]map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["volumeAttachment"]["volumeId"] != "volume-id" {
				t.Errorf("wrong attachment request: %v", body)
			}
			fmt.Fprint(w, `{"volumeAttachment":{"id":"volume-id","volumeId":"volume-id","serverId":"server-id"}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	for _, args := range [][]string{{"create", "ci-docker", "--size", "40", "--wait"}, {"attach", "volume-id", "--server", "server-id", "--wait"}} {
		output, err := executeTestCommand(newVolumeCmd(provider), args...)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output, "volume-id") {
			t.Fatalf("missing resource ID: %s", output)
		}
	}
	if len(requests) != 4 {
		t.Fatalf("unexpected requests: %v", requests)
	}
}

func TestResourceDiscovery(t *testing.T) {
	for _, resource := range []string{"image", "flavor", "network"} {
		t.Run(resource, func(t *testing.T) {
			requests := 0
			provider := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				switch resource {
				case "image":
					if r.URL.Path != "/image/v2/images" || (requests == 1 && r.URL.Query().Get("status") != "active") {
						t.Errorf("wrong image request: %s", r.URL)
					}
					// Exercise pagination so IDs on later pages are discoverable.
					if requests == 1 {
						fmt.Fprint(w, `{"images":[{"id":"first-id","name":"first"}],"next":"/v2/images?marker=first-id"}`)
					} else {
						fmt.Fprint(w, `{"images":[{"id":"wanted-id","name":"trixie"}]}`)
					}
				case "flavor":
					if r.URL.Path != "/compute/flavors/detail" {
						t.Errorf("wrong flavor request: %s", r.URL)
					}
					fmt.Fprint(w, `{"flavors":[{"id":"wanted-id","name":"g4.cores4.ram8.disk20"}]}`)
				case "network":
					if r.URL.Path != "/network/v2.0/networks" {
						t.Errorf("wrong network request: %s", r.URL)
					}
					fmt.Fprint(w, `{"networks":[{"id":"wanted-id","name":"project-network"}]}`)
				}
			})
			output, err := executeTestCommand(newResourceListCmd(resource, provider), "list")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, "wanted-id") {
				t.Fatalf("missing resource: %s", output)
			}
		})
	}
}

func TestCreateFailurePreservesResourceID(t *testing.T) {
	provider := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"server":{"id":"failed-server-id"}}`)
		} else {
			fmt.Fprint(w, `{"server":{"id":"failed-server-id","status":"ERROR"}}`)
		}
	})
	output, err := executeTestCommand(newComputeCreateCmd(provider), "ci", "--image=i", "--flavor=f", "--network=n", "--wait")
	if err == nil || !strings.Contains(err.Error(), "failed-server-id") || !strings.Contains(err.Error(), "ERROR") {
		t.Fatalf("expected failure with instance ID and state: %v", err)
	}
	if !strings.Contains(output, "failed-server-id") {
		t.Fatalf("lost instance ID after failed wait: %s", output)
	}
}

func TestProvisionValidation(t *testing.T) {
	provider := func(cmd *cobra.Command) (*gophercloud.ProviderClient, error) {
		t.Fatal("invalid arguments should fail before authentication")
		return nil, nil
	}
	for _, tc := range []struct {
		cmd     *cobra.Command
		args    []string
		message string
	}{
		{newComputeCreateCmd(provider), []string{"ci-runner"}, "IDs are required"},
		{newComputeCreateCmd(provider), []string{"-bad"}, "unknown shorthand"},
		{newComputeCreateCmd(provider), []string{"bad.name"}, "DNS label"},
		{newComputeCreateCmd(provider), []string{"ci", "--image=i", "--flavor=f", "--network=n", "--timeout=0s"}, "positive"},
		{newVolumeCmd(provider), []string{"create", "ci", "--size=-1"}, "positive"},
		{newVolumeCmd(provider), []string{"attach", "volume-id"}, "--server"},
	} {
		_, err := executeTestCommand(tc.cmd, tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("%v: expected %q, got %v", tc.args, tc.message, err)
		}
	}
}

func TestWaitForResource(t *testing.T) {
	err := waitForResource(context.Background(), "ACTIVE", func(context.Context) (string, error) { return "ERROR", nil })
	if err == nil || !strings.Contains(err.Error(), "ERROR") {
		t.Fatalf("expected terminal failure: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	err = waitForResource(ctx, "available", func(context.Context) (string, error) { return "creating", nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout: %v", err)
	}
}

func TestMissingProjectCredentials(t *testing.T) {
	_, err := projectCredentials("mwcli", map[string]config.CloudVPSProject{})
	if err == nil || !strings.Contains(err.Error(), "auth add --project mwcli") {
		t.Fatalf("expected setup instructions, got %v", err)
	}
}

func TestInstanceSSHArgs(t *testing.T) {
	command := []string{"printf", "%s\\n", "a b", "it's literal", "$(echo unsafe); *", ""}
	args, err := instanceSSHArgs("ci-runner", "mwcli", "alice", "bastion", false, command)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []string{"-l", "alice", "-J", "bastion", "ci-runner.mwcli.eqiad1.wikimedia.cloud"}
	if !reflect.DeepEqual(args[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("unexpected SSH arguments: %v", args)
	}
	// Exercise the remote shell interpretation: metacharacters and empty args survive literally.
	output, err := exec.Command("sh", "-c", args[len(args)-1]).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != strings.Join(command[2:], "\n")+"\n" {
		t.Fatalf("arguments changed by shell: %q", output)
	}
	args, err = instanceSSHArgs("ci-runner", "mwcli", "", "", false, nil)
	if err != nil || args[0] != "-t" {
		t.Fatalf("interactive session missing terminal: %v, %v", args, err)
	}
	if _, err := instanceSSHArgs("-oProxyCommand=bad", "mwcli", "", "", false, nil); err == nil {
		t.Fatal("unsafe hostname accepted")
	}
}
