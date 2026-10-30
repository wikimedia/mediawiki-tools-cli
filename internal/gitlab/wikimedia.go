// gitlab in internal utils is functionality talking to gitlab
package gitlab

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

var (
	wikimediav4ApiURL = "https://gitlab.wikimedia.org/api/v4/"
	projectID         = 16 // ID 16 is repos/releng/cli
	outOs             = runtime.GOOS
	arch              = runtime.GOARCH
)

const (
	wikimediaAPIRequestTimeout = 30 * time.Second
	wikimediaReleasesPerPage   = 100
)

func wikimediaClient() *gitlab.Client {
	httpClient := http.Client{
		Timeout: wikimediaAPIRequestTimeout,
	}
	git, err := gitlab.NewClient(
		"",
		gitlab.WithBaseURL(wikimediav4ApiURL),
		gitlab.WithCustomRetryMax(1),
		gitlab.WithHTTPClient(
			&httpClient,
		),
	)
	if err != nil {
		panic(err)
	}
	git.UserAgent = cli.HTTPUserAgent
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

	return releasesBetweenTags(releases, from, to)
}

func releasesBetweenTags(releases []*gitlab.Release, from, to string) ([]*gitlab.Release, error) {
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

	// Return releases traversed when moving from -> to, excluding from and including to.
	// Releases are ordered newest to oldest, so index direction depends on version direction.
	if start > end {
		return releases[end:start], nil
	}
	if start < end {
		return releases[start+1 : end+1], nil
	}

	return []*gitlab.Release{}, nil
}

func RelengCliGetReleases() ([]*gitlab.Release, error) {
	// Ask for the largest supported page. The default page is needlessly small
	// for release history, and the unqualified endpoint can be very slow on
	// Wikimedia GitLab.
	releases, _, err := wikimediaClient().Releases.ListReleases(projectID, &gitlab.ListReleasesOptions{
		ListOptions: gitlab.ListOptions{PerPage: wikimediaReleasesPerPage},
	})
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

	return releaseBinary(release)
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

	return releaseBinary(release)
}

// RelengCliNightlyBinaryURL finds a successful nightly rather than whichever
// weekly/Dependabot pipeline most recently ran on the shared scheduled ref.
func RelengCliNightlyBinaryURL() (string, error) {
	client := wikimediaClient()
	pipelines, _, err := client.Pipelines.ListProjectPipelines(projectID, &gitlab.ListProjectPipelinesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 1},
		Ref:         gitlab.Ptr("scheduled"), Source: gitlab.Ptr("schedule"),
		Name: gitlab.Ptr("Nightly CLI build"), Status: gitlab.Ptr(gitlab.Success),
		OrderBy: gitlab.Ptr("id"), Sort: gitlab.Ptr("desc"),
	})
	if err != nil {
		return "", fmt.Errorf("find nightly pipeline: %w", err)
	}
	if len(pipelines) == 0 {
		return "", fmt.Errorf("no successful nightly build is available yet")
	}
	pipeline := pipelines[0]
	if pipeline.Ref != "scheduled" || pipeline.Status != "success" || pipeline.Name != "Nightly CLI build" {
		return "", fmt.Errorf("GitLab returned an unexpected pipeline for the nightly query")
	}
	jobs, _, err := client.Jobs.ListPipelineJobs(projectID, pipeline.ID, &gitlab.ListJobsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100}, Scope: gitlab.Ptr([]gitlab.BuildStateValue{gitlab.Success}),
	})
	if err != nil {
		return "", fmt.Errorf("find nightly artifact: %w", err)
	}
	for _, job := range jobs {
		if job.Name != "build-release" || job.Status != "success" || job.Ref != "scheduled" || job.ArtifactsFile.Size <= 0 {
			continue
		}
		if job.ArtifactsExpireAt != nil && job.ArtifactsExpireAt.Before(time.Now()) {
			return "", fmt.Errorf("the latest successful nightly artifact has expired; wait for the next nightly build")
		}
		filename := releaseBinaryName(cli.BinaryNames()[0], "nightly", outOs, arch)
		return fmt.Sprintf("%sprojects/%d/jobs/%d/artifacts/_release/nightly/%s", wikimediav4ApiURL, projectID, job.ID, filename), nil
	}
	return "", fmt.Errorf("the latest successful nightly pipeline has no downloadable build-release artifact")
}

func releaseBinary(release *gitlab.Release) (*gitlab.ReleaseLink, error) {
	var expected []string
	for _, name := range cli.BinaryNames() {
		filename := releaseBinaryName(name, release.TagName, outOs, arch)
		expected = append(expected, filename)
		for _, link := range release.Assets.Links {
			if link.Name == filename {
				return link, nil
			}
		}
	}
	return nil, errors.New("no binary release found: " + strings.Join(expected, " or "))
}

func releaseBinaryName(name, tag, platform, architecture string) string {
	filename := name + "_" + tag + "_" + platform + "_" + architecture
	if platform == "windows" {
		filename += ".exe"
	}
	return filename
}
