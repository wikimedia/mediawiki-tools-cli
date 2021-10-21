/*Package gitlab in internal utils is functionality talking to gitlab

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package gitlab

import (
	"errors"
	"net/http"
	"runtime"
	"time"

	gitlab "github.com/xanzy/go-gitlab"
)

var (
	wikimediav4ApiURL = "https://gitlab.wikimedia.org/api/v4/"
	os                = runtime.GOOS
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

/*RelengCliLatestRelease from gitlab.*/
func RelengCliLatestRelease() (*gitlab.Release, error) {
	// ID 16 is releng/mwcli
	releases, _, err := wikimediaClient().Releases.ListReleases(16, nil)
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
	return "mw_" + tagName + "_" + os + "_" + arch
}
