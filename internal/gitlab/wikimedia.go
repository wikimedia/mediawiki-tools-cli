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
	"fmt"
	"net/http"
	"runtime"
	"time"

	gitlab "github.com/xanzy/go-gitlab"
)

var wikimediav4ApiURL = "https://gitlab.wikimedia.org/api/v4/"
var os = runtime.GOOS
var arch = runtime.GOARCH

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

/*RelengCliLatestRelease from gitlab*/
func RelengCliLatestRelease() (*gitlab.Release, error) {
	// ID 16 in releng/mwcli
	releases, response, err := wikimediaClient().Releases.ListReleases(16, nil)
	if err != nil {
		fmt.Println(response.Status)
		fmt.Println(response.Body)
		panic(err)
	}

	if len(releases) < 1 {
		return nil, errors.New("this gitlab project has no releases")
	}
	return releases[0], nil
}

/*RelengCliLatestReleaseBinary from gitlab for this OS and ARCH*/
func RelengCliLatestReleaseBinary() (*gitlab.ReleaseLink, error) {
	release, err := RelengCliLatestRelease()
	if err != nil {
		return nil, err
	}

	// Look for something like mw_v0.1.0-dev.20210920.1_linux_386
	lookFor := "mw_" + release.TagName + "_" + os + "_" + arch

	for _, link := range release.Assets.Links {
		if link.Name == lookFor {
			return link, nil
		}
	}
	return nil, errors.New("no binary release found matching VERSION, OS and ARCH")
}
