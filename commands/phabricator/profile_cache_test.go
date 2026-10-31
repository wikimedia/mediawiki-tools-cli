package phabricator

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"
)

func TestProfileCacheScope(t *testing.T) {
	root := t.TempDir()
	base := PhabConfig{SiteName: "personal", Username: "Addshore", URL: defaultPhabricatorURL, CachePath: root}
	personal := base
	applyDefaultCachePath(&personal)
	for _, field := range []string{"profile", "account", "endpoint", "project"} {
		t.Run(field, func(t *testing.T) {
			other := base
			switch field {
			case "profile":
				other.SiteName = "readonly"
			case "account":
				other.Username = "Addbot"
			case "endpoint":
				other.URL = "https://phabricator.example.org"
			case "project":
				other.DefaultProject = "another-project"
			}
			applyDefaultCachePath(&other)
			if other.CachePath == personal.CachePath {
				t.Fatal("distinct profile identities share a cache")
			}
			if filepath.Dir(filepath.Dir(other.CachePath)) != root {
				t.Fatal("configured cache root was not preserved")
			}
		})
	}
	// Reopening the same profile after a token update must keep its own cache.
	again := base
	again.Key = "replacement-fixture"
	applyDefaultCachePath(&again)
	if again.CachePath != personal.CachePath {
		t.Fatal("cache identity depends on the credential value")
	}
	base.CachePath = ""
	personal, other := base, base
	other.SiteName = "bot"
	applyDefaultCachePath(&personal)
	applyDefaultCachePath(&other)
	if personal.CachePath == other.CachePath {
		t.Fatal("default cache roots do not isolate profiles")
	}
}

func TestTransactionsCannotCrossProfileCaches(t *testing.T) {
	root := t.TempDir()
	personal := PhabConfig{SiteName: "personal", Username: "Addshore", URL: defaultPhabricatorURL, CachePath: root}
	bot := PhabConfig{SiteName: "bot", Username: "Addbot", URL: defaultPhabricatorURL, CachePath: root}
	applyDefaultCachePath(&personal)
	applyDefaultCachePath(&bot)
	first := newConduitClient(&personal)
	first.cache.transactions.set("tx:PHID-TASK-fixture", []phabTransaction{}, ttlTransactions)
	first.cache.flush()

	denied := errors.New("fixture: account cannot access this task")
	requests := 0
	transport := authTransport(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, denied
	})
	second := newConduitClient(&bot)
	second.httpClient.Transport = transport
	if _, err := second.getTransactions("PHID-TASK-fixture"); !errors.Is(err, denied) || requests != 1 {
		t.Fatalf("bot bypassed its own authorization check: requests=%d, error=%v", requests, err)
	}

	reopened := newConduitClient(&personal)
	reopened.httpClient.Transport = transport
	if _, err := reopened.getTransactions("PHID-TASK-fixture"); err != nil || requests != 1 {
		t.Fatalf("personal cache was not reusable: requests=%d, error=%v", requests, err)
	}
}
