package phabricator

import (
	"testing"
	"time"
)

func TestSuggestedTokenName(t *testing.T) {
	now := time.Date(2026, time.October, 29, 15, 4, 5, 0, time.UTC)

	tests := []struct {
		name     string
		hostname string
		want     string
	}{
		{name: "hostname", hostname: "devbox", want: "mwcli 2026-10-29 devbox"},
		{name: "empty hostname", hostname: "  ", want: "mwcli 2026-10-29 local"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := suggestedTokenName(now, test.hostname); got != test.want {
				t.Errorf("suggestedTokenName(%q) = %q, want %q", test.hostname, got, test.want)
			}
		})
	}
}
