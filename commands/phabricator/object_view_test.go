package phabricator

import "testing"

func TestObjectReferenceParsing(t *testing.T) {
	for raw, want := range map[string]string{"T123": "T123", "p123": "P123", "https://phabricator.wikimedia.org/P123#content": "P123", "https://phabricator.example/phab/T123/": "T123"} {
		ref, err := parseObjectReference(raw)
		if err != nil || ref.Ref != want {
			t.Fatalf("%s: %+v %v", raw, ref, err)
		}
	}
	for _, raw := range []string{"123", "T0", "P-1", "T123junk", " P123", "T1/P2", "D123", "https://name:pass@phabricator.example/P123", "https://phabricator.example/P123?other=1", "ftp://phabricator.example/P123", "https://phabricator.example/%50%31%32%33", "T99999999999999999999999999999999"} {
		if _, err := parseObjectReference(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestObjectURLMustMatchProfile(t *testing.T) {
	for _, tc := range []struct {
		raw, endpoint string
		valid         bool
	}{
		{"P123", "https://phabricator.example", true},
		{"https://phabricator.example/P123", "https://phabricator.example", true},
		{"https://other.example/P123", "https://phabricator.example", false},
		{"http://phabricator.example/P123", "https://phabricator.example", false},
		{"https://phabricator.example/other/P123", "https://phabricator.example", false},
		{"https://phabricator.example/phab/P123", "https://phabricator.example/phab", true},
	} {
		ref, err := parseObjectReference(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := ref.checkEndpoint(tc.endpoint); (err == nil) != tc.valid {
			t.Fatalf("%s -> %s: %v", tc.raw, tc.endpoint, err)
		}
	}
}
