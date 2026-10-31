package config

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

type statusFixture struct {
	values               map[string]string
	failRead, failDelete bool
}

func (s *statusFixture) Set(service, account, value string) error {
	s.values[service+account] = value
	return nil
}

func (s *statusFixture) Get(service, account string) (string, error) {
	if s.failRead {
		return "", errors.New("sensitive-provider-error")
	}
	return s.values[service+account], nil
}

func (s *statusFixture) Delete(service, account string) error {
	if s.failDelete {
		return errors.New("sensitive-provider-error")
	}
	delete(s.values, service+account)
	return nil
}

func TestSecretStatusChecksTemporaryNativeItemOnly(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	oldDetect, oldStore := statusKeyring, statusNativeStore
	t.Cleanup(func() { statusKeyring, statusNativeStore = oldDetect, oldStore })
	statusKeyring = func(context.Context) secrets.Availability {
		return secrets.Availability{Available: true, Detail: "test native store"}
	}
	client := &statusFixture{values: map[string]string{"unrelated-item": "private-fixture"}}
	statusNativeStore = func() secrets.Store { return client }
	for _, args := range [][]string{{"keyring"}, {"keyring", "--check"}} {
		command := NewSecretStatusCmd()
		output := &bytes.Buffer{}
		command.SetOut(output)
		command.SetErr(output)
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), "private-fixture") || len(client.values) != 1 {
			t.Fatal("status exposed or retained credentials")
		}
	}
	for _, args := range [][]string{{"--check"}, {"keepassxc", "--check"}, {"other"}} {
		command := NewSecretStatusCmd()
		command.SetOut(&bytes.Buffer{})
		command.SetErr(&bytes.Buffer{})
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Fatal("invalid check accepted")
		}
	}
}

func TestNativeProbeCleansUpAfterReadFailure(t *testing.T) {
	client := &statusFixture{values: map[string]string{}, failRead: true}
	err := probeCredentialStore(client)
	if err == nil || len(client.values) != 0 || strings.Contains(err.Error(), "sensitive-provider-error") {
		t.Fatal("probe failed to clean up or exposed provider output")
	}
	client.failDelete = true
	err = probeCredentialStore(client)
	if err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatal("missing cleanup diagnostic")
	}
}
