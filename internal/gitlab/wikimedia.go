// gitlab in internal utils is functionality talking to gitlab
package gitlab

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
)

var (
	wikimediav4ApiURL = "https://gitlab.wikimedia.org/api/v4/"
	projectID         = 16 // ID 16 is releng/mwcli
	outOs             = runtime.GOOS
	arch              = runtime.GOARCH
)

func wikimediaClient() *gitlab.Client {
	httpClient := http.Client{
		Timeout: 2 * time.Second,
	}
	git, err := gitlab.NewClient(
		"",
		gitlab.WithBaseURL(wikimediav4ApiURL),
		gitlab.WithoutRetries(),
		gitlab.WithHTTPClient(
			&httpClient,
		),
	)
	if err != nil {
		panic(err)
	}
	return git
}

func RelengCliGetRelease(name string) (*gitlab.Release, error) {
	release, _, err := wikimediaClient().Releases.GetRelease(projectID, name, nil)
	return release, err
}

func RelengCliGetReleasesBetweenTags(from, to string) ([]*gitlab.Release, error) {
	logrus.Tracef("Getting releases between tags: %s and %s", from, to)

	// Get all releases
	releases, err := RelengCliGetReleases()
	if err != nil {
		return nil, err
	}

	// Assume they are in release order.
	// Remove everything from the start, up until the value of to
	// Then remove everything from the end, after the value of from
	// This will leave us with the releases between the two tags
	start := -1
	end := -1
	for i, release := range releases {
		if release.TagName == to {
			end = i
		}
		if release.TagName == from {
			start = i
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("could not find start tag: %s", from)
	}
	if end == -1 {
		return nil, fmt.Errorf("could not find end tag: %s", to)
	}
	return releases[end:start], nil
}

func RelengCliGetReleases() ([]*gitlab.Release, error) {
	releases, _, err := wikimediaClient().Releases.ListReleases(projectID, nil)
	return releases, err
}

/*RelengCliLatestRelease from gitlab.*/
func RelengCliLatestRelease() (*gitlab.Release, error) {
	releases, err := RelengCliGetReleases()
	if err != nil {
		return nil, err
	}

	if len(releases) < 1 {
		return nil, errors.New("this gitlab project has no releases")
	}
	return releases[0], nil
}

/*RelengCliLatestReleaseBinary from gitlab for this OS and ARCH.*/
func RelengCliLatestReleaseBinary() (*gitlab.ReleaseLink, error) {
	release, err := RelengCliLatestRelease()
	if err != nil {
		return nil, err
	}

	lookFor := binaryName(release.TagName)
	for _, link := range release.Assets.Links {
		if link.Name == lookFor {
			return link, nil
		}
	}
	return nil, errors.New("no binary release found: " + lookFor)
}

func RelengCliRelease(tagName string) (*gitlab.Release, error) {
	release, _, err := wikimediaClient().Releases.GetRelease(16, tagName, nil)
	return release, err
}

func RelengCliReleaseBinary(tagName string) (*gitlab.ReleaseLink, error) {
	release, err := RelengCliRelease(tagName)
	if err != nil {
		return nil, err
	}

	lookFor := binaryName(release.TagName)
	for _, link := range release.Assets.Links {
		if link.Name == lookFor {
			return link, nil
		}
	}
	return nil, errors.New("no binary release found: " + lookFor)
}

func binaryName(tagName string) string {
	// something like mw_v0.5.0_linux_386
	if outOs == "windows" {
		return "mw_" + tagName + "_" + outOs + "_" + arch + ".exe"
	}
	return "mw_" + tagName + "_" + outOs + "_" + arch
}
