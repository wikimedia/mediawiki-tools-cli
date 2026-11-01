package wikiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func bulkResults(t *testing.T, output string) []BulkResult {
	t.Helper()
	rows := []BulkResult{}
	decoder := json.NewDecoder(strings.NewReader(output))
	for {
		var row BulkResult
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err, output)
		}
		rows = append(rows, row)
	}
	return rows
}

func TestBulkPartialFailureRevisionChecksAndNDJSON(t *testing.T) {
	revision := int64(12)
	posts, tokens := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form
		if r.Method == "POST" {
			posts++
		}
		switch q.Get("action") {
		case "wbgetentities":
			id := q.Get("ids")
			rev := revision
			if id == "Q2" {
				rev = 20
			}
			fmt.Fprintf(w, `{"entities":{%q:{"id":%q,"type":"item","lastrevid":%d}}}`, id, id, rev)
		case "query":
			if q.Get("meta") == "tokens" {
				tokens++
				fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
			} else {
				fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Sandbox","lastrevid":5}]}}`)
			}
		case "wbsetlabel":
			if q.Get("id") == "Q2" {
				fmt.Fprint(w, `{"error":{"code":"permissiondenied","info":"Denied"}}`)
			} else {
				if q.Get("baserevid") != "12" {
					t.Error(q)
				}
				revision++
				fmt.Fprint(w, `{"success":1,"entity":{"id":"Q1","lastrevid":13}}`)
			}
		case "edit":
			if q.Get("baserevid") != "5" || q.Get("text") != "New text" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"edit":{"result":"Success","newrevid":6}}`)
		default:
			t.Error(q)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	input := `{"record_id":"ok","operation":"term-change","kind":"item","entity":"Q1","term":"label","change":"set","language":"en","values":["New"],"baserevid":12,"summary":"Fixture"}
{"record_id":"stale","operation":"term-change","kind":"item","entity":"Q1","term":"label","change":"set","language":"en","values":["Stale"],"baserevid":12,"summary":"Fixture"}
{bad json}
{"operation":"term-change","kind":"item","entity":"Q1","term":"label","change":"set","language":"en","values":["Unknown flag"],"baserevid":13,"summary":"Fixture","unknown":true}
{"operation":"term-change","kind":"item","entity":"Q2","term":"label","change":"set","language":"en","values":["Denied"],"baserevid":20,"summary":"Fixture"}
{"operation":"page-put","title":"Sandbox","text":"New text","baserevid":5,"summary":"Fixture"}`
	var output bytes.Buffer
	stats, err := client.RunBulk(context.Background(), strings.NewReader(input), &output, BulkOptions{})
	var failed *BulkError
	if !errors.As(err, &failed) || stats.Succeeded != 2 || stats.Failed != 4 || stats.Processed != 6 || posts != 3 || tokens != 3 {
		t.Fatalf("stats=%+v posts=%d tokens=%d err=%v", stats, posts, tokens, err)
	}
	results := bulkResults(t, output.String())
	if len(results) != 6 || results[0].Status != "success" || results[1].Error.Code != "conflict" || results[2].Error.Code != "invalid_record" || results[3].Error.Code != "invalid_record" || results[4].Error.Code != "permissiondenied" || results[4].Error.Ambiguous || results[5].Status != "success" {
		t.Fatalf("incorrect per-record results: %+v", results)
	}
}

func TestBulkDryRunAndStopOnError(t *testing.T) {
	writes, tokens := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Method == "POST" {
			writes++
		}
		if r.Form.Get("meta") == "tokens" {
			tokens++
		}
		if r.Form.Get("action") == "wbgetentities" {
			fmt.Fprint(w, `{"entities":{"Q1":{"id":"Q1","type":"item","lastrevid":12}}}`)
		} else {
			fmt.Fprint(w, `{"query":{"pages":[{"title":"New page","missing":true}]}}`)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	record := `{"operation":"term-change","kind":"item","entity":"Q1","term":"label","change":"set","language":"en","values":["New"],"baserevid":12,"summary":"Fixture"}`
	input := record + "\n" + `{"operation":"page-put","title":"New page","text":"","createonly":true,"baserevid":0,"summary":"Create"}`
	var output bytes.Buffer
	stats, err := client.RunBulk(context.Background(), strings.NewReader(input), &output, BulkOptions{DryRun: true})
	if err != nil || stats.Planned != 2 || writes != 0 || tokens != 0 {
		t.Fatalf("dry run: %+v %v", stats, err)
	}
	output.Reset()
	bad := strings.Replace(record, `"baserevid":12`, `"baserevid":11`, 1)
	stats, err = client.RunBulk(context.Background(), strings.NewReader(bad+"\n"+record), &output, BulkOptions{StopOnError: true})
	if err == nil || stats.Processed != 1 || len(bulkResults(t, output.String())) != 1 || writes != 0 || tokens != 0 {
		t.Fatalf("stop-on-error: %+v %v", stats, err)
	}
}

func TestBulkAmbiguousPOSTIsNotRetried(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.Form.Get("action") {
		case "wbgetentities":
			fmt.Fprint(w, `{"entities":{"Q1":{"id":"Q1","type":"item","lastrevid":12}}}`)
		case "query":
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
		default:
			posts++
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = connection.Close()
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	input := `{"operation":"term-change","kind":"item","entity":"Q1","term":"label","change":"set","language":"en","values":["New"],"baserevid":12,"summary":"Fixture"}`
	var output bytes.Buffer
	stats, err := client.RunBulk(context.Background(), strings.NewReader(input), &output, BulkOptions{})
	rows := bulkResults(t, output.String())
	if err == nil || stats.Failed != 1 || posts != 1 || len(rows) != 1 || !rows[0].Error.Ambiguous {
		t.Fatalf("ambiguous POST: posts=%d stats=%+v rows=%+v err=%v", posts, stats, rows, err)
	}
}
