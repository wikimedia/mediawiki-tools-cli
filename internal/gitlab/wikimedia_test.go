package gitlab

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestLatestReleaseBinary(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		if req.URL.Path == "/api/v4/projects/16/releases" {
			b, err := os.ReadFile("testdata/wikimedia_test_data.json")
			if err != nil {
				panic(err)
			}
			rw.Write(b)
		}
	}))
	// Close the server when test finishes
	defer server.Close()

	// Override the URL with our test server
	wikimediav4ApiURL = server.URL + "/api/v4/"
	outOs = "fakeOS"
	arch = "fakeArch"

	tests := []struct {
		name    string
		fakeOS  string
		want    *gitlab.ReleaseLink
		wantErr bool
	}{
		{
			name:   "valid, first link",
			fakeOS: "firstOS",
			want: &gitlab.ReleaseLink{
				ID:             1,
				Name:           "mw_REL_TAG_NAME_firstOS_fakeArch",
				URL:            "someUrl1",
				DirectAssetURL: "someDirectUrl1",
				External:       true,
				LinkType:       "other",
			},
		},
		{
			name:   "valid, second link",
			fakeOS: "secondOS",
			want: &gitlab.ReleaseLink{
				ID:             2,
				Name:           "mw_REL_TAG_NAME_secondOS_fakeArch",
				URL:            "someUrl2",
				DirectAssetURL: "someDirectUrl2",
				External:       true,
				LinkType:       "other",
			},
		},
		{
			name:    "invalid",
			fakeOS:  "thirdOS",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outOs = tt.fakeOS
			got, err := RelengCliLatestReleaseBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("LatestReleaseBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("LatestReleaseBinary() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestReleasesBetweenTags_UpgradeDirection(t *testing.T) {
	releases := []*gitlab.Release{
		{TagName: "v0.32.0"},
		{TagName: "v0.31.0"},
		{TagName: "v0.30.0"},
	}

	got, err := releasesBetweenTags(releases, "v0.31.0", "v0.32.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].TagName != "v0.32.0" {
		t.Fatalf("expected v0.32.0, got %s", got[0].TagName)
	}
}

func TestReleasesBetweenTags_DowngradeDirection(t *testing.T) {
	releases := []*gitlab.Release{
		{TagName: "v0.32.0"},
		{TagName: "v0.31.0"},
		{TagName: "v0.30.0"},
	}

	got, err := releasesBetweenTags(releases, "v0.32.0", "v0.31.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].TagName != "v0.31.0" {
		t.Fatalf("expected v0.31.0, got %s", got[0].TagName)
	}
}

func TestReleasesBetweenTags_MissingTags(t *testing.T) {
	releases := []*gitlab.Release{
		{TagName: "v0.32.0"},
		{TagName: "v0.31.0"},
	}

	_, err := releasesBetweenTags(releases, "v0.30.0", "v0.31.0")
	if err == nil {
		t.Fatal("expected error when from tag is missing")
	}

	_, err = releasesBetweenTags(releases, "v0.31.0", "v0.30.0")
	if err == nil {
		t.Fatal("expected error when to tag is missing")
	}
}

func TestReleasesBetweenTags_SameTag(t *testing.T) {
	releases := []*gitlab.Release{
		{TagName: "v0.32.0"},
		{TagName: "v0.31.0"},
	}

	got, err := releasesBetweenTags(releases, "v0.31.0", "v0.31.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 releases, got %d", len(got))
	}
}
