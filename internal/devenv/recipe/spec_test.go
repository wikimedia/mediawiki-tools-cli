package recipe

import "testing"

func TestParseMinimalSpec(t *testing.T) {
	raw := []byte(`
type: mwcli.dev/recipe
version: 0.1
name: tiny
services:
  - name: mediawiki
`)

	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if s.Type != TypeMWCLIDevRecipe {
		t.Fatalf("unexpected type: %s", s.Type)
	}
	if len(s.Services) != 1 || s.Services[0].State != "started" {
		t.Fatalf("expected one started service, got %+v", s.Services)
	}
}

func TestParseRejectsUnknownType(t *testing.T) {
	raw := []byte(`
type: unknown/recipe
version: 1
services:
  - name: mediawiki
`)

	_, err := Parse(raw)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseRejectsUnsupportedDBType(t *testing.T) {
	raw := []byte(`
type: mwcli.dev/recipe
version: 1
sites:
  - dbname: en
    dbtype: oracle
`)

	_, err := Parse(raw)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
