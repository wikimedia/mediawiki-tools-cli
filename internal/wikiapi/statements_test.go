package wikiapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const statementEntityFixture = `{"entities":{"Q1":{"id":"Q1","type":"item","lastrevid":12,"claims":{"P31":[{"id":"opaque-statement","type":"statement","rank":"preferred","mainsnak":{"snaktype":"value","property":"P31","datatype":"wikibase-item","datavalue":{"type":"wikibase-entityid","value":{"entity-type":"item","id":"Q5"}}},"qualifiers":{"P1":[{"hash":"qualifier-hash","snaktype":"value","property":"P1","datatype":"string","datavalue":{"type":"string","value":"Old qualifier"}}]},"references":[{"hash":"reference-hash","snaks":{"P1":[{"snaktype":"value","property":"P1","datatype":"string","datavalue":{"type":"string","value":"Source"}}]}}],"extension-field":{"kept":true}}]}}}}`

func TestTypedSnakValuesAndRawEscape(t *testing.T) {
	for _, tc := range []struct{ datatype, scalar, data string }{
		{"string", "Literal | text", ""},
		{"external-id", "A-42", ""},
		{"url", "https://example.org/source", ""},
		{"commonsMedia", "Example.jpg", ""},
		{"wikibase-item", "Q5", ""},
		{"wikibase-property", "P31", ""},
		{"quantity", "12.5", ""},
		{"quantity", "", `{"amount":"+12","unit":"http://www.wikidata.org/entity/Q11573","lowerBound":"+11","upperBound":"+13"}`},
		{"monolingualtext", "", `{"text":"Example","language":"en"}`},
		{"time", "", `{"time":"+2026-01-02T00:00:00Z","precision":11,"timezone":0,"before":0,"after":0,"calendarmodel":"http://www.wikidata.org/entity/Q1985727"}`},
		{"globe-coordinate", "", `{"latitude":51.5,"longitude":-0.1,"precision":0.001,"globe":"http://www.wikidata.org/entity/Q2"}`},
	} {
		t.Run(tc.datatype+tc.scalar, func(t *testing.T) {
			if _, err := NewSnak("P1", tc.datatype, "value", tc.scalar, json.RawMessage(tc.data)); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct{ datatype, scalar, data string }{
		{"wikibase-item", "P31", ""},
		{"url", "file:///tmp/private", ""},
		{"string", "", ""},
		{"quantity", "NaN", ""},
		{"quantity", "", `{"amount":"+12","unit":"1","lowerBound":"+13"}`},
		{"time", "", `{"time":"+2026-02-31T00:00:00Z","precision":11,"calendarmodel":"http://www.wikidata.org/entity/Q1985727"}`},
		{"time", "", `{"time":"+2026-00-00T00:00:00Z","precision":11,"calendarmodel":"http://www.wikidata.org/entity/Q1985727"}`},
		{"globe-coordinate", "", `{"latitude":95,"longitude":1,"precision":1,"globe":"http://www.wikidata.org/entity/Q2"}`},
		{"monolingualtext", "", `{"text":"Example","language":"en|de"}`},
	} {
		if _, err := NewSnak("P1", tc.datatype, "value", tc.scalar, json.RawMessage(tc.data)); err == nil {
			t.Errorf("invalid typed value accepted: %+v", tc)
		}
	}
	if _, err := NewSnak("P1", "", "novalue", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSnak("P1", "string", "somevalue", "Should not exist", nil); err == nil {
		t.Fatal("somevalue with value accepted")
	}
	var snak Snak
	_ = json.Unmarshal([]byte(`{"property":"P1","snaktype":"value","datatype":"entity-schema","datavalue":{"type":"string","value":"E1"}}`), &snak)
	if ValidateSnak(snak, false) == nil || ValidateSnak(snak, true) != nil {
		t.Fatal("raw native escape was not explicit")
	}
}

func TestStatementPlansPreserveStateAndCheckMembershipAndRevision(t *testing.T) {
	posts, tokens := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form
		switch q.Get("action") {
		case "wbgetentities":
			fmt.Fprint(w, statementEntityFixture)
		case "query":
			tokens++
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
		default:
			posts++
			if r.Method != "POST" || q.Get("baserevid") != "12" || q.Get("summary") != "Fixture edit" || q.Get("token") != "fixture" {
				t.Error(q)
			}
			if q.Get("action") == "wbsetclaim" {
				var statement Statement
				_ = json.Unmarshal([]byte(q.Get("claim")), &statement)
				if rawString(statement["rank"]) != "preferred" || len(statement["qualifiers"]) == 0 || len(statement["references"]) == 0 || len(statement["extension-field"]) == 0 {
					t.Error("typed value update lost other statement fields")
				}
			}
			fmt.Fprint(w, `{"success":1}`)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	snak, _ := NewSnak("P31", "wikibase-item", "value", "Q6", nil)
	plan, err := client.PlanStatement(context.Background(), "item", "Q1", StatementChange{Operation: "update", ID: "opaque-statement", Snak: snak}, 12, false)
	if err != nil {
		t.Fatal(err)
	}
	if posts != 0 || tokens != 0 {
		t.Fatal("planning performed a write or CSRF request")
	}
	if _, err = client.ExecuteWikibasePlan(context.Background(), plan, EditOptions{Summary: "Fixture edit"}); err != nil {
		t.Fatal(err)
	}
	_, err = client.PlanStatement(context.Background(), "item", "Q1", StatementChange{Operation: "remove", ID: "opaque-statement"}, 11, false)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if _, err = client.PlanStatement(context.Background(), "item", "Q1", StatementChange{Operation: "remove", ID: "other-entity-statement"}, 12, false); err == nil {
		t.Fatal("cross-entity statement accepted")
	}
	qualifier, _ := NewSnak("P1", "string", "value", "A | B", nil)
	qplan, err := client.PlanQualifier(context.Background(), "item", "Q1", "opaque-statement", "add", "", qualifier, 12, false)
	if err != nil {
		t.Fatal(err)
	}
	if qplan.fields.Get("value") != `"A | B"` {
		t.Fatal(qplan.fields)
	}
	reference := Reference{Snaks: map[string][]Snak{"P1": {qualifier}}, Order: []string{"P1"}}
	rplan, err := client.PlanReference(context.Background(), "item", "Q1", "opaque-statement", "add", "", reference, 12, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*WikibaseEditPlan{qplan, rplan} {
		if _, err = client.ExecuteWikibasePlan(context.Background(), p, EditOptions{Summary: "Fixture edit"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, remove := range []bool{false, true} {
		hash := "wrong"
		if remove {
			hash = "qualifier-hash"
		}
		_, err = client.PlanQualifier(context.Background(), "item", "Q1", "opaque-statement", "remove", hash, nil, 12, false)
		if (err == nil) != remove {
			t.Fatalf("qualifier ownership validation: %v", err)
		}
	}
	for _, remove := range []bool{false, true} {
		hash := "wrong"
		if remove {
			hash = "reference-hash"
		}
		_, err = client.PlanReference(context.Background(), "item", "Q1", "opaque-statement", "remove", hash, Reference{}, 12, false)
		if (err == nil) != remove {
			t.Fatalf("reference ownership validation: %v", err)
		}
	}
	before := tokens
	if _, err = client.ExecuteWikibasePlan(context.Background(), plan, EditOptions{BaseRevision: 99}); err == nil {
		t.Fatal("execution silently replaced requested revision")
	}
	if tokens != before || posts != 3 {
		t.Fatal("invalid operations performed additional writes")
	}
}

func TestStatementValidationRejectsInvalidRankAndQualifierProperty(t *testing.T) {
	for _, data := range []string{`{"type":5}`, `{"rank":false}`, `{"rank":"unrecognized"}`, `{"mainsnak":{"property":"P1","snaktype":"novalue"},"qualifiers":{"P2":[{"property":"P1","snaktype":"novalue"}]}}`} {
		var statement Statement
		_ = json.Unmarshal([]byte(data), &statement)
		if ValidateStatement(statement, false) == nil {
			t.Errorf("invalid statement accepted: %s", data)
		}
	}
	var statement Statement
	_ = json.Unmarshal([]byte(`{"mainsnak":{"property":"P1","snaktype":"novalue"},"references":[{"snaks":{}}]}`), &statement)
	if err := ValidateStatement(statement, false); err == nil || !strings.Contains(err.Error(), "nonempty") {
		t.Fatal("empty reference accepted", err)
	}
}
