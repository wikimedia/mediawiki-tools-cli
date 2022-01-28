package hosts

import (
	"testing"
)

func TestLocalhostIps(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "localhost has 127.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := IPs("localhost")

			if !splitContains(got, "127.0.0.1") {
				t.Errorf("LocalhostIps() = doesn't container 127.0.0.1 and probably should, got: %v", got)
				return
			}
		})
	}
}

func splitContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
