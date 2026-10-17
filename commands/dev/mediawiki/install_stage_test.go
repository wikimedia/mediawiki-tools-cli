package mediawiki

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunInstallStage(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, wantErr := range []error{nil, failure} {
		var out bytes.Buffer
		calls := 0
		err := runInstallStage(&out, "Install database", func() error { calls++; return wantErr })
		if calls != 1 || !errors.Is(err, wantErr) {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
		if !strings.HasPrefix(out.String(), "\n==> Install database\n") {
			t.Fatal(out.String())
		}
		if strings.Contains(out.String(), "Completed:") != (wantErr == nil) {
			t.Fatal(out.String())
		}
		if err != nil && !strings.Contains(err.Error(), "Install database") {
			t.Fatal(err)
		}
	}
}
