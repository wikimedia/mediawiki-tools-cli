package wiki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

const cliStatementFixture = `{"id":"opaque-statement","type":"statement","rank":"preferred","mainsnak":{"snaktype":"value","property":"P31","datatype":"wikibase-item","datavalue":{"type":"wikibase-entityid","value":{"id":"Q5","entity-type":"item"}}},"qualifiers":{"P1":[{"hash":"qual-hash","snaktype":"value","property":"P1","datatype":"string","datavalue":{"type":"string","value":"Old"}}]},"references":[{"hash":"ref-hash","snaks":{"P1":[{"snaktype":"value","property":"P1","datatype":"string","datavalue":{"type":"string","value":"Source"}}]}}]}`

func TestStatementQualifierReferenceCLIAndDryRun(t *testing.T) {
	isolateWikiProfiles(t)
	writes, tokens := 0, 0
	lastAction := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form
		switch q.Get("action") {
		case "wbgetentities":
			id := q.Get("ids")
			kind := "item"
			if strings.HasPrefix(id, "P") {
				kind = "property"
			}
			fmt.Fprintf(w, `{"entities":{%q:{"id":%q,"type":%q,"lastrevid":12,"claims":{"P31":[%s]}}}}`, id, id, kind, cliStatementFixture)
		case "query":
			tokens++
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
		default:
			writes++
			lastAction = q.Get("action")
			if q.Get("baserevid") != "12" || q.Get("token") != "fixture" || r.Method != "POST" {
				t.Error(q)
			}
			switch lastAction {
			case "wbsetclaim":
				var statement map[string]json.RawMessage
				_ = json.Unmarshal([]byte(q.Get("claim")), &statement)
				if string(statement["rank"]) != `"preferred"` || len(statement["references"]) == 0 {
					t.Error("typed update lost state")
				}
			case "wbremovequalifiers":
				if q.Get("qualifiers") != "qual-hash" {
					t.Error(q)
				}
			case "wbremovereferences":
				if q.Get("references") != "ref-hash" {
					t.Error(q)
				}
			}
			fmt.Fprint(w, `{"success":1,"entity":{"id":"Q1","lastrevid":13}}`)
		}
	}))
	defer server.Close()
	common := []string{"--anon", "--wiki", server.URL + "/w/api.php"}
	run := func(args []string, input string) (string, error) {
		return runWikiProfile(t, append(append([]string{"ext", "wikibase"}, args...), common...), input)
	}
	for _, args := range [][]string{{"item", "statement", "list", "Q1", "--property", "P31"}, {"property", "statement", "get", "P31", "opaque-statement"}, {"item", "statement", "qualifier", "list", "Q1", "opaque-statement"}, {"item", "statement", "reference", "list", "Q1", "opaque-statement"}} {
		if out, err := run(args, ""); err != nil || out == "" {
			t.Fatal(out, err)
		}
	}
	if out, err := run([]string{"item", "statement", "add", "Q1", "P31", "--datatype", "wikibase-item", "--value", "Q5", "--dry-run"}, ""); err != nil || !strings.Contains(out, `"action":"wbeditentity"`) {
		t.Fatal(out, err)
	}
	reference := `{"snaks":{"P1":[{"property":"P1","snaktype":"value","datatype":"string","datavalue":{"type":"string","value":"New source"}}]},"snaks-order":["P1"]}`
	for _, tc := range []struct {
		args          []string
		input, action string
	}{
		{[]string{"item", "statement", "add", "Q1", "P31", "--datatype", "wikibase-item", "--value", "Q5"}, "", "wbeditentity"},
		{[]string{"item", "statement", "update", "Q1", "opaque-statement", "P31", "--datatype", "wikibase-item", "--value", "Q6"}, "", "wbsetclaim"},
		{[]string{"item", "statement", "qualifier", "add", "Q1", "opaque-statement", "P1", "--datatype", "string", "--value", "New qualifier"}, "", "wbsetqualifier"},
		{[]string{"item", "statement", "qualifier", "remove", "Q1", "opaque-statement", "qual-hash"}, "", "wbremovequalifiers"},
		{[]string{"item", "statement", "reference", "add", "Q1", "opaque-statement", "--input", "-"}, reference, "wbsetreference"},
		{[]string{"item", "statement", "reference", "remove", "Q1", "opaque-statement", "ref-hash"}, "", "wbremovereferences"},
		{[]string{"item", "statement", "remove", "Q1", "opaque-statement"}, "", "wbremoveclaims"},
	} {
		if _, err := run(tc.args, tc.input); err != nil || lastAction != tc.action {
			t.Fatalf("%v: action=%s err=%v", tc.args, lastAction, err)
		}
	}
	if writes != 7 || tokens != 7 {
		t.Fatal(writes, tokens)
	}
	beforeTokens, beforeWrites := tokens, writes
	for _, args := range [][]string{{"item", "statement", "remove", "Q1", "foreign"}, {"item", "statement", "remove", "Q1", "opaque-statement", "--baserevid", "11"}, {"item", "statement", "qualifier", "remove", "Q1", "opaque-statement", "foreign-hash"}, {"item", "statement", "reference", "remove", "Q1", "opaque-statement", "foreign-hash"}, {"item", "statement", "add", "Q1", "P1", "--datatype", "url", "--value", "file:///tmp/private"}} {
		if _, err := run(args, ""); err == nil {
			t.Errorf("unsafe edit accepted: %v", args)
		}
	}
	raw := `{"mainsnak":{"property":"P1","snaktype":"value","datatype":"entity-schema","datavalue":{"type":"string","value":"E1"}}}`
	if _, err := run([]string{"item", "statement", "add", "Q1", "--input", "-"}, raw); err == nil {
		t.Fatal("unsupported datatype accepted without raw escape")
	}
	if _, err := run([]string{"item", "statement", "add", "Q1", "--input", "-", "--raw-json", "--dry-run"}, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := run([]string{"item", "statement", "reference", "add", "Q1", "opaque-statement", "--input", "-"}, strings.Replace(reference, `"snaks-order"`, `"unexpected"`, 1)); err == nil {
		t.Fatal("unknown reference field silently discarded")
	}
	if tokens != beforeTokens || writes != beforeWrites {
		t.Fatal("invalid operations or dry-run fetched CSRF/wrote")
	}
}

func TestDiffAndFileCLIOutputs(t *testing.T) {
	isolateWikiProfiles(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != "GET" {
			t.Error("read command wrote")
		}
		if q.Get("action") == "compare" {
			fmt.Fprint(w, `{"compare":{"fromrevid":1,"torevid":2,"body":"<tr><td><pre>@@ -1 +1 @@\n-old\n+new\n</pre></td></tr>"}}`)
			return
		}
		if q.Get("prop") == "imageinfo" {
			fmt.Fprint(w, `{"query":{"pages":[{"title":"File:Example.jpg","ns":6,"missing":true,"imagerepository":"shared","imageinfo":[{"mime":"image/jpeg","url":"https://assets.invalid/never-fetched.jpg"}]}]}}`)
			return
		}
		if q.Get("prop") != "fileusage" || q.Get("funamespace") != "0" {
			t.Error(q)
		}
		fmt.Fprint(w, `{"query":{"pages":[{"title":"File:Example.jpg","ns":6,"missing":true,"fileusage":[{"pageid":1,"ns":0,"title":"Earth"}]}]}}`)
	}))
	defer server.Close()
	common := []string{"--anon", "--wiki", server.URL + "/w/api.php"}
	for _, tc := range []struct {
		args []string
		want string
	}{{[]string{"page", "diff", "Earth"}, "-old\n+new"}, {[]string{"page", "diff", "rev:1", "rev:2", "--output", "jq", "--format", ".torevid"}, "2"}, {[]string{"file", "info", "Example.jpg", "--output", "jq", "--format", ".imageinfo[0].mime"}, "image/jpeg"}, {[]string{"file", "usage", "Example.jpg", "--namespace", "0", "--output", "titles"}, "Earth"}} {
		out, err := runWikiProfile(t, append(tc.args, common...), "")
		if err != nil || !strings.Contains(out, tc.want) {
			t.Fatalf("%v: %q %v", tc.args, out, err)
		}
	}
}

func TestBulkCLIKeepsNDJSONOnStdoutAndFailsForPartialBatch(t *testing.T) {
	isolateWikiProfiles(t)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.Form.Get("action") {
		case "wbgetentities":
			fmt.Fprint(w, `{"entities":{"Q1":{"id":"Q1","type":"item","lastrevid":12}}}`)
		case "query":
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
		default:
			writes++
			fmt.Fprint(w, `{"success":1,"entity":{"id":"Q1"}}`)
		}
	}))
	defer server.Close()
	valid := `{"operation":"term-change","kind":"item","entity":"Q1","term":"label","change":"set","language":"en","values":["New"],"baserevid":12,"summary":"Fixture"}`
	invalid := `{"operation":"unsupported","baserevid":12,"summary":"Fixture"}`
	for _, dry := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		cmd := NewWikiCmd()
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		input := valid + "\n" + invalid
		if dry {
			input = valid
		}
		cmd.SetIn(strings.NewReader(input))
		args := []string{"bulk", "apply", "--input", "-", "--anon", "--wiki", server.URL + "/w/api.php"}
		if dry {
			args = append(args, "--dry-run")
		}
		cmd.SetArgs(args)
		err := cmd.Execute()
		if (err == nil) != dry {
			t.Fatalf("unexpected bulk exit: %v", err)
		}
		decoder := json.NewDecoder(&stdout)
		rows := []wikiapi.BulkResult{}
		for {
			var row wikiapi.BulkResult
			e := decoder.Decode(&row)
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatalf("non-NDJSON output: %v", e)
			}
			rows = append(rows, row)
		}
		if dry {
			if len(rows) != 1 || rows[0].Status != "planned" {
				t.Fatal(rows)
			}
		} else if len(rows) != 2 || rows[0].Status != "success" || rows[1].Error == nil {
			t.Fatal(rows)
		}
	}
	if writes != 1 {
		t.Fatalf("dry-run wrote or operation was retried: %d", writes)
	}
}
