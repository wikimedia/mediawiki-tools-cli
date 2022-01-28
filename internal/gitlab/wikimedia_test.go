package gitlab

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	gitlab "github.com/xanzy/go-gitlab"
)

func TestLatestReleaseBinary(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		if req.URL.Path == "/api/v4/projects/16/releases" {
			b, err := ioutil.ReadFile("testdata/wikimedia_test_data.json")
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
	os = "fakeOS"
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
			os = tt.fakeOS
			got, err := RelengCliLatestReleaseBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("LatestReleaseBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LatestReleaseBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}
