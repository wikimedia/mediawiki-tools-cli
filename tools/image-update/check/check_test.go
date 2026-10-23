package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestNewerTagsUseVersionOrder(t *testing.T) {
	got := keepNewerTags("1.0.0", []string{"2.10.0", "invalid", "2.9.0", "latest", "0.9.0"})
	want := []string{"2.9.0", "2.10.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTagFilterMatchesWholeTag(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		pattern string
		want    []string
	}{
		{"Redis family", []string{"8.6.7", "8.10.2", "8.6.7-alpine"}, `8\.6\.\d+`, []string{"8.6.7"}},
		{"old Redis filter", []string{"8.6.7"}, `6\.\d+$`, []string{}},
		{"Shellbox PHP version", []string{"2026-10-13-100406-media83", "2026-10-13-100406-media85"}, `\d{4}-\d{2}-\d{2}-\d{6}-media83`, []string{"2026-10-13-100406-media83"}},
		{"stable DNS", []string{"5.9.0", "5.9.1-snapshot-jre"}, `5\.\d+\.\d+`, []string{"5.9.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keepTagsMatchingRegex(tt.tags, tt.pattern); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Authorization") != "Bearer anonymous-fixture" {
			t.Error("missing registry request headers")
		}
		if r.URL.Query().Get("last") == "one" {
			fmt.Fprint(w, `{"tags":["two"]}`)
		} else {
			w.Header().Set("Link", `</tags?last=one>; rel="next"`)
			fmt.Fprint(w, `{"tags":["one"]}`)
		}
	}))
	defer server.Close()
	got, err := registryTags(server.URL+"/tags", "anonymous-fixture")
	if err != nil || !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestRegistryFailureIsNotNoUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"errors":[{"code":"DENIED"}]}`)
	}))
	defer server.Close()
	if _, err := registryTags(server.URL); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected HTTP failure, got %v", err)
	}
}

func TestRegistryPaginationLoopFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `</tags>; rel="next"`)
		fmt.Fprint(w, `{"tags":["one"]}`)
	}))
	defer server.Close()
	if _, err := registryTags(server.URL + "/tags"); err == nil || !strings.Contains(err.Error(), "loop") {
		t.Fatalf("expected pagination failure, got %v", err)
	}
}
