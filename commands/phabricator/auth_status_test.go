package phabricator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestStatusChecksProfileIdentity(t *testing.T) {
	for _, expected := range []string{"Addshore", "addshore", "Addbot"} {
		t.Run(expected, func(t *testing.T) {
			isolatedAuthConfig(t)
			mockLoginIdentity(t)
			disk := koanf.New(".")
			for key, value := range map[string]string{
				"phabricator.profiles.fixture.username": expected,
				"phabricator.profiles.fixture.url":      defaultPhabricatorURL,
				"phabricator.profiles.fixture.key":      "personal-fixture",
			} {
				if err := disk.Set(key, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := mwconfig.PutDiskConfig(disk); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			cmd := NewPhabricatorCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"auth", "status", "--profile", "fixture"})
			err := cmd.Execute()
			if expected == "Addbot" {
				if err == nil || !strings.Contains(err.Error(), "authenticated as Addshore, expected Addbot") {
					t.Fatalf("wrong-account result: %v", err)
				}
			} else if err != nil || !strings.Contains(output.String(), "Authenticated as Addshore") {
				t.Fatalf("matching identity rejected: %v", err)
			}
			if strings.Contains(output.String(), "personal-fixture") {
				t.Fatal("status exposed the token")
			}
		})
	}
}
