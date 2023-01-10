package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	utilstrings "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
	"gopkg.in/yaml.v2"
)

type Data struct {
	Images []struct {
		Image        string `yaml:"image"`
		RequireRegex string `yaml:"requireRegex,omitempty"`
		NoCheck      bool   `yaml:"noCheck,omitempty"`
	} `yaml:"images"`
	Directories []string `yaml:"directories"`
	Files       []string `yaml:"files"`
}

func getData() Data {
	var data Data
	content, err := ioutil.ReadFile("tools/image-update/data.yml")
	if err != nil {
		panic(err.Error())
	}
	err = yaml.Unmarshal(content, &data)
	if err != nil {
		log.Fatal("Failed to parse data file ", err)
	}
	return data
}

func main() {
	data := getData()

	delayOutputSkipped := []string{}
	delayOutputNoNewtags := []string{}
	delayOutputCommands := []string{}

	for _, imageData := range data.Images {
		image := imageData.Image
		r := regexp.MustCompile(`(\$\{.*:\-)?([\w\/\-\.]+):([\w\-\_\.]+)(\})?`)
		regexSplit := r.FindStringSubmatch(image)
		imageName := regexSplit[2]
		imageTag := regexSplit[3]
		humanCheckURL := humanURLForImageName(imageName)

		if imageData.NoCheck {
			delayOutputSkipped = append(delayOutputSkipped, fmt.Sprintf("%s, currently using %s", humanCheckURL, image))
			continue
		}

		// Filter all tags
		tags := tagsForImageName(imageName)
		tags = keepNewerTags(imageTag, tags)
		tags = keepTagsMatchingRegex(tags, imageData.RequireRegex)

		if len(tags) > 0 {
			fmt.Println("-------------------------------------------------------")
			fmt.Println("Apparently there are newer tags available. You might want to consider updating the image!")
			fmt.Printf("%s\n", humanCheckURL)
			fmt.Printf("%s ->> %v\n", imageTag, tags)

			lastTag := tags[len(tags)-1]
			delayOutputCommands = append(delayOutputCommands, fmt.Sprintf("go run tools/image-update/update/update.go %s %s", image, imageName+":"+lastTag))
		} else {
			message := fmt.Sprintf("No newer tags found for %s", imageName)
			if imageData.RequireRegex != "" {
				message += fmt.Sprintf(" matching regex %s", imageData.RequireRegex)
			}
			delayOutputNoNewtags = append(delayOutputNoNewtags, message)
		}
	}

	fmt.Println("-------------------------------------------------------")
	for _, v := range delayOutputNoNewtags {
		fmt.Println(v)
	}
	fmt.Println("-------------------------------------------------------")
	fmt.Println("Images that were not checked")
	for _, v := range delayOutputSkipped {
		fmt.Println(v)
	}
	fmt.Println("-------------------------------------------------------")
	fmt.Println("Commands to update images")
	for _, v := range delayOutputCommands {
		fmt.Println(v)
	}

	// write commands to file (if there are any)
	if len(delayOutputCommands) != 0 {
		err := ioutil.WriteFile("tools/image-update/.update.sh", []byte(strings.Join(delayOutputCommands, "\n")+"\n"), 0o755)
		if err != nil {
			panic(err)
		}
		fmt.Println("-------------------------------------------------------")
		fmt.Println("Commands to update images written to tools/image-update/.update.sh")
	}
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
		if utilstrings.StringInSlice(tag, skipTags) {
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

func humanURLForImageName(imageName string) string {
	if stringStartsWith(imageName, "docker-registry.wikimedia.org") {
		return "https://" + imageName + "/tags"
	}
	if strings.Contains(imageName, "/") {
		return "https://hub.docker.com/r/" + imageName + "/tags"
	}
	return "https://hub.docker.com/_/" + imageName + "/tags"
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

	body, readErr := ioutil.ReadAll(res.Body)
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
