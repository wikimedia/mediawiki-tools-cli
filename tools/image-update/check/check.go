package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
	"gopkg.in/yaml.v2"
)

type Data struct {
	Images      []ImageData `yaml:"images"`
	ImageGroups []struct {
		Name           string      `yaml:"name"`
		SameTagMatcher string      `yaml:"sameTagMatcher"`
		Images         []ImageData `yaml:"images"`
	} `yaml:"imageGroups"`
	Directories []string `yaml:"directories"`
	Files       []string `yaml:"files"`
}

type ImageData struct {
	Image        string `yaml:"image"`
	RequireRegex string `yaml:"requireRegex,omitempty"`
	NoCheck      bool   `yaml:"noCheck,omitempty"`
}

type CommandToRun struct {
	Name        string
	Description string
	Command     string
}

type CheckResult struct {
	Skipped   []ImageData
	NoNewTags []ImageData
	Commands  []CommandToRun
}

func getData() Data {
	var data Data
	content, err := os.ReadFile("tools/image-update/data.yml")
	if err != nil {
		panic(err.Error())
	}
	err = yaml.Unmarshal(content, &data)
	if err != nil {
		logrus.Fatal("Failed to parse data file ", err)
	}
	return data
}

func main() {
	data := getData()
	result := CheckResult{}

	for _, imageData := range data.Images {
		fmt.Println("Checking", imageData.Image)
		image := imageData.Image
		imageName, imageTag := imageData.nameAndTagFromTaggedName()

		if imageData.NoCheck {
			result.Skipped = append(result.Skipped, imageData)
			continue
		}

		tagsOfInterest := keepTagsMatchingRegex(keepNewerTags(imageTag, tagsForImageName(imageName)), imageData.RequireRegex)

		if len(tagsOfInterest) > 0 {
			lastTag := tagsOfInterest[len(tagsOfInterest)-1]
			result.Commands = append(result.Commands, CommandToRun{
				Name:        imageName,
				Description: fmt.Sprintf("Bump %s from %s to %s", imageName, imageTag, lastTag),
				Command:     fmt.Sprintf("go run tools/image-update/update/update.go %s %s", image, imageName+":"+lastTag),
			})
		} else {
			result.NoNewTags = append(result.NoNewTags, imageData)
		}
	}

	for _, imageGroup := range data.ImageGroups {
		imageUpdatePairs := []string{}
		oneNewtag := true
		newTag := ""
		for _, imageData := range imageGroup.Images {
			fmt.Println("Checking", imageData.Image)
			image := imageData.Image
			imageName, imageTag := imageData.nameAndTagFromTaggedName()

			if imageData.NoCheck {
				result.Skipped = append(result.Skipped, imageData)
				continue
			}

			tagsOfInterest := keepTagsMatchingRegex(keepNewerTags(imageTag, tagsForImageName(imageName)), imageData.RequireRegex)

			if len(tagsOfInterest) > 0 {
				lastTag := tagsOfInterest[len(tagsOfInterest)-1]
				imageUpdatePairs = append(imageUpdatePairs, image, imageName+":"+lastTag)

				r := regexp.MustCompile(imageGroup.SameTagMatcher)
				regexSplit := r.FindStringSubmatch(lastTag)
				if newTag == "" || regexSplit[1] == newTag {
					newTag = regexSplit[1]
				} else {
					oneNewtag = false
				}
			} else {
				result.NoNewTags = append(result.NoNewTags, imageData)
			}
		}
		if len(imageUpdatePairs) > 0 {
			description := ""
			if oneNewtag {
				description = fmt.Sprintf("Bump %s image group to %s", imageGroup.Name, newTag)
			} else {
				description = fmt.Sprintf("Bump %s image group", imageGroup.Name)
			}

			result.Commands = append(result.Commands, CommandToRun{
				Name:        imageGroup.Name,
				Description: description,
				Command:     "go run tools/image-update/update/update.go " + strings.Join(imageUpdatePairs, " "),
			})
		}
	}
	if len(result.Commands) > 0 {
		fmt.Println("There are newer tags available. You might want to consider updating the image!")
		for _, v := range result.Commands {
			fmt.Printf("Update: %s\n", v.Name)
			fmt.Printf("Command: `%s`\n", v.Command)
		}
	}
	if len(result.NoNewTags) > 0 {
		fmt.Println("There are no newer tags available for the following images:")
		for _, v := range result.NoNewTags {
			fmt.Printf("Image: %s\n", v.Image)
			fmt.Printf("URL: %s\n", v.humanURLForImageName())
			if v.RequireRegex != "" {
				fmt.Printf("Require Regex: %s", v.RequireRegex)
			}
		}
	}
	if len(result.Skipped) > 0 {
		fmt.Println("The following images were skipped:")
		for _, v := range result.Skipped {
			fmt.Printf("Image: %s\n", v.Image)
			fmt.Printf("URL: %s\n", v.humanURLForImageName())
		}
	}

	if len(result.Commands) > 0 {
		bashOutput := ""
		for _, v := range result.Commands {
			bashOutput += fmt.Sprintf(`%s`+"\n", v.Command)
		}
		err := os.WriteFile("tools/image-update/.update.sh", []byte(bashOutput), 0o755) // #nosec G306
		if err != nil {
			panic(err)
		}
		fmt.Println("Commands to update images written to tools/image-update/.update.sh")
	}

	if len(result.Commands) > 0 {
		gitlabOutput := ""
		for _, v := range result.Commands {
			gitlabOutput += fmt.Sprintf(`        - NAME: "%s"`+"\n", v.Name)
			gitlabOutput += fmt.Sprintf(`          DESCRIPTION: "%s"`+"\n", v.Description)
			gitlabOutput += fmt.Sprintf(`          COMMAND: "%s"`+"\n", v.Command)
		}
		err := os.WriteFile("tools/image-update/.gitlab.update.yaml", []byte(gitlabOutput), 0o755) // #nosec G306
		if err != nil {
			panic(err)
		}
		fmt.Println("A snippet for use in Gitlab CI is written to tools/image-update/.gitlab.update.sh")
	}
}

func (i ImageData) nameAndTagFromTaggedName() (string, string) {
	r := regexp.MustCompile(`(\$\{.*:\-)?([\w\/\-\.]+):([\w\-\_\.]+)(\})?`)
	regexSplit := r.FindStringSubmatch(i.Image)
	imageName := regexSplit[2]
	imageTag := regexSplit[3]
	return imageName, imageTag
}

func (i ImageData) humanURLForImageName() string {
	imageName, _ := i.nameAndTagFromTaggedName()
	if stringStartsWith(imageName, "docker-registry.wikimedia.org") {
		return "https://" + imageName + "/tags"
	}
	if strings.Contains(imageName, "/") {
		return "https://hub.docker.com/r/" + imageName + "/tags"
	}
	return "https://hub.docker.com/_/" + imageName + "/tags"
}

func keepNewerTags(currentTag string, allTags []string) []string {
	// There will be no newer tags if the current tag is "latest"
	if currentTag == "latest" {
		return []string{}
	}

	// Parse the currently used version
	current, err := version.NewVersion(currentTag)
	if err != nil {
		panic(err)
	}

	// Is the currently used version a WMF security release?
	// If so, add the original release to a list to skip...
	regex := `\-s\d+$`
	skipTags := []string{}
	if stringMatchesRegex(currentTag, regex) {
		noSecurityTag := regexp.MustCompile(regex).ReplaceAllString(currentTag, "")
		skipTags = append(skipTags, noSecurityTag)
	}

	// Parse and compare all fonud tags
	newerTags := []string{}
	for _, tag := range allTags {
		if stringsutil.StringInSlice(tag, skipTags) {
			continue
		}
		compare, err := version.NewVersion(tag)
		if err != nil {
			continue
		}
		if current.LessThan(compare) {
			newerTags = append(newerTags, tag)
		}
	}

	return newerTags
}

func keepTagsMatchingRegex(tags []string, regex string) []string {
	if regex == "" {
		return tags
	}
	matchingTags := []string{}
	for _, tag := range tags {
		if stringMatchesRegex(tag, regex) {
			matchingTags = append(matchingTags, tag)
		}
	}
	return matchingTags
}

func stringMatchesRegex(str string, regex string) bool {
	r := regexp.MustCompile(regex)
	return r.MatchString(str)
}

func tagsForImageName(imageName string) []string {
	// Use the google code mirror, as the real registry requires auth...
	apiURL := "https://mirror.gcr.io/v2/library/" + imageName + "/tags/list"

	if stringStartsWith(imageName, "docker-registry.wikimedia.org") {
		apiURL = "https://docker-registry.wikimedia.org/v2/" + strings.Replace(imageName, "docker-registry.wikimedia.org/", "", 1) + "/tags/list/"
	}

	v2Res := v2Response{}
	_, err := jsonFromURL(apiURL, &v2Res)
	if err != nil {
		logrus.Error(err)
		return []string{}
	}
	return v2Res.Tags
}

type v2Response struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func jsonFromURL(url string, unmarshalTo interface{}) (interface{}, error) {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "mwcli-tools-image-check")

	res, getErr := client.Do(req)
	if getErr != nil {
		return nil, getErr
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}

	jsonErr := json.Unmarshal(body, &unmarshalTo)
	if jsonErr != nil {
		return nil, jsonErr
	}

	return unmarshalTo, nil
}

func stringStartsWith(s string, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}
