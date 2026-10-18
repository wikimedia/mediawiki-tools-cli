package secrets

import (
	"strings"
	"testing"
)

func TestEndpoint(t *testing.T) {
	for _, raw := range []string{"http://example.org", "https://user:pass@example.org", "https://example.org?token=x", "https://example.org/#token", "file:///tmp/secret"} {
		if _, err := Endpoint(raw); err == nil {
			t.Errorf("accepted unsafe endpoint %q", raw)
		}
	}
	for _, raw := range []string{"https://example.org", "http://localhost:1234", "http://127.0.0.1:1234", "http://[::1]:1234"} {
		if _, err := Endpoint(raw); err != nil {
			t.Errorf("rejected %q: %v", raw, err)
		}
	}
}

func TestReadInput(t *testing.T) {
	value, err := ReadInput(strings.NewReader(" space $pecial secret \r\n"))
	if err != nil || value != " space $pecial secret " {
		t.Fatalf("input changed: %q %v", value, err)
	}
	for _, input := range []string{"", "\n", strings.Repeat("x", 65537)} {
		if _, err := ReadInput(strings.NewReader(input)); err == nil {
			t.Fatal("accepted invalid input")
		}
	}
}
