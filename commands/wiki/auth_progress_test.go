package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrdinaryOAuthAPIProgressUsesStderrAndHonorsCancellation(t *testing.T) {
	isolateWikiProfiles(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"query":{"userinfo":{"id":1,"name":"Fixture"}}}`)
	}))
	defer server.Close()
	seedAPIOAuth(t, server.URL+"/w/api.php")
	for _, test := range []struct{ anon, canceled bool }{{false, false}, {false, true}, {true, false}} {
		cmd := NewWikiCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		args := []string{"api", "query", "-f", "meta=userinfo"}
		if test.anon {
			args = append(args, "--anon")
		}
		cmd.SetArgs(args)
		ctx, cancel := context.WithCancel(context.Background())
		if test.canceled {
			cancel()
		}
		cmd.SetContext(ctx)
		err := cmd.Execute()
		cancel()
		if test.canceled {
			if !errors.Is(err, context.Canceled) || stdout.Len() != 0 {
				t.Fatal(stdout.String(), err)
			}
		} else if err != nil || !json.Valid(stdout.Bytes()) {
			t.Fatal(stdout.String(), err)
		}
		progress := strings.Contains(stderr.String(), "Reading saved OAuth credentials")
		if progress == test.anon || strings.Contains(stderr.String(), "api-fixture") {
			t.Fatal("missing/misrouted or sensitive diagnostics", stderr.String())
		}
	}
	if requests != 2 {
		t.Fatal(requests)
	}
}
