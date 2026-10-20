package gitlab

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestLatestReleaseBinary(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		if req.URL.Path == "/api/v4/projects/16/releases" {
			if req.URL.Query().Get("per_page") != "100" {
				t.Errorf("expected per_page=100, got %q", req.URL.Query().Get("per_page"))
			}
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

func TestRelengCliGetReleasesRetriesServerErrors(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if requests.Add(1) == 1 {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		b, err := os.ReadFile("testdata/wikimedia_test_data.json")
		if err != nil {
			t.Errorf("read release fixture: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = rw.Write(b)
	}))
	defer server.Close()

	previousURL := wikimediav4ApiURL
	wikimediav4ApiURL = server.URL + "/api/v4/"
	t.Cleanup(func() { wikimediav4ApiURL = previousURL })

	releases, err := RelengCliGetReleases()
	if err != nil {
		t.Fatalf("RelengCliGetReleases() error = %v", err)
	}
	if len(releases) == 0 {
		t.Fatal("expected releases after retry")
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
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

func TestReleaseBinaryAcceptsBothNamesAndSelectsPlatform(t *testing.T) {
	previousOS, previousArch := outOs, arch
	t.Cleanup(func() { outOs, arch = previousOS, previousArch })
	outOs, arch = "linux", "amd64"
	for _, name := range []string{"mw", "wm"} {
		right := &gitlab.ReleaseLink{Name: name + "_v1.2.3_linux_amd64", URL: "https://example.org/binary"}
		release := &gitlab.Release{TagName: "v1.2.3"}
		release.Assets.Links = []*gitlab.ReleaseLink{{Name: name + "_v1.2.3_linux_arm64"}, {Name: right.Name + ".sha256"}, right}
		got, err := releaseBinary(release)
		if err != nil || got != right {
			t.Fatalf("%s asset: got %v, %v", name, got, err)
		}
	}
	release := &gitlab.Release{TagName: "v1.2.3"}
	release.Assets.Links = []*gitlab.ReleaseLink{{Name: "mw_v1.2.3_linux_arm64"}, {Name: "wm_v1.2.3_windows_amd64.exe"}}
	if _, err := releaseBinary(release); err == nil {
		t.Fatal("wrong platform accepted")
	}
	outOs = "windows"
	want := &gitlab.ReleaseLink{Name: "wm_v1.2.3_windows_amd64.exe"}
	release.Assets.Links = append(release.Assets.Links, want)
	got, err := releaseBinary(release)
	// Either matching Windows link is acceptable; Linux links must not win.
	if err != nil || got.Name != want.Name {
		t.Fatalf("Windows asset: %v, %v", got, err)
	}
}
