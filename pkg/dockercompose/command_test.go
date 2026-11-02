package dockercompose

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

func TestRunAttachedWithIOUsesProvidedStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("input\n")
	command := Command{Cmd: exec.Command("/bin/sh", "-c", "read value; printf 'out:%s' \"$value\"; printf 'err' >&2")}
	if err := command.RunAttachedWithIO(stdin, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "out:input"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "err"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestShouldForceDockerDefaultPlatform(t *testing.T) {
	origGOOS := runtimeGOOS
	origGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		runtimeGOARCH = origGOARCH
	})

	_ = os.Unsetenv("DOCKER_DEFAULT_PLATFORM")

	tests := []struct {
		name          string
		goos          string
		goarch        string
		defaultPlat   string
		wantShouldSet bool
	}{
		{name: "linux arm64 with no override", goos: "linux", goarch: "arm64", defaultPlat: "", wantShouldSet: true},
		{name: "linux arm with no override", goos: "linux", goarch: "arm", defaultPlat: "", wantShouldSet: true},
		{name: "linux amd64 with no override", goos: "linux", goarch: "amd64", defaultPlat: "", wantShouldSet: false},
		{name: "darwin arm64 with no override", goos: "darwin", goarch: "arm64", defaultPlat: "", wantShouldSet: false},
		{name: "linux arm64 with explicit override", goos: "linux", goarch: "arm64", defaultPlat: "linux/arm64", wantShouldSet: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtimeGOOS = tc.goos
			runtimeGOARCH = tc.goarch
			if tc.defaultPlat == "" {
				if err := os.Unsetenv("DOCKER_DEFAULT_PLATFORM"); err != nil {
					t.Fatalf("unset env: %v", err)
				}
			} else {
				t.Setenv("DOCKER_DEFAULT_PLATFORM", tc.defaultPlat)
			}

			got := shouldForceDockerDefaultPlatform()
			if got != tc.wantShouldSet {
				t.Fatalf("shouldForceDockerDefaultPlatform() = %v, want %v", got, tc.wantShouldSet)
			}
		})
	}
}
