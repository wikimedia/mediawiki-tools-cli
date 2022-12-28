package hosts

import (
	"testing"
)

func TestHostIps(t *testing.T) {
	tests := []struct {
		name string
		host string
		has  string
	}{
		{
			name: "localhost has 127.0.0.1",
			host: "localhost",
			has:  "127.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := IPs(tt.host)

			if !splitContains(got, tt.has) {
				t.Errorf("IPs() = for %v doesn't contain %v and probably should, got: %v", tt.host, tt.has, got)
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
