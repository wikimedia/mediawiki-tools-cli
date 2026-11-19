package cmd

import "testing"

func TestMCPProfileBindings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		services []string
		bindings []string
		profile  string
		valid    bool
	}{
		{"anonymous", []string{"phab", "wiki"}, nil, "", true},
		{"named", []string{"phab", "wiki"}, []string{"phab=work"}, "work", true},
		{"unknown service", []string{"unknown"}, nil, "", false},
		{"duplicate service", []string{"phab", "phab"}, nil, "", false},
		{"blank profile", []string{"phab"}, []string{"phab="}, "", false},
		{"unqualified profile", []string{"phab"}, []string{"work"}, "", false},
		{"duplicate profile", []string{"phab"}, []string{"phab=work", "phab=other"}, "", false},
		{"disabled service", []string{"wiki"}, []string{"phab=work"}, "", false},
		{"unsupported profile", []string{"wiki"}, []string{"wiki=work"}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile, err := mcpPhabricatorProfile(tc.services, nil, tc.bindings)
			if (err == nil) != tc.valid || profile != tc.profile {
				t.Fatalf("profile = %q, error = %v", profile, err)
			}
		})
	}
}
