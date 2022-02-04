package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type DockerComposeConfig struct {
	Services map[string]struct {
		Image string
	}
}

func main() {
	dirPath := "./internal/mwdd/files/embed"

	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		logrus.Fatal(err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yml" || file.IsDir() {
			continue
		}

		yamlFile, err := ioutil.ReadFile(dirPath + "/" + file.Name())
		if err != nil {
			logrus.Printf("yamlFile.Get err   #%v ", err)
		}

		c := &DockerComposeConfig{}

		err = yaml.Unmarshal(yamlFile, c)
		if err != nil {
			logrus.Fatalf("Unmarshal: %v", err)
		}

		for serviceName, service := range c.Services {
			r := regexp.MustCompile(`(\$\{.*:\-)?([\w\/\-\.]+):([\w\.]+)(\})?`)
			regexSplit := r.FindStringSubmatch(service.Image)
			imageName := regexSplit[2]
			imageTag := regexSplit[3]
			fmt.Println("-------------------------------------------------------")
			fmt.Printf("Checking %s service %s using image %s\n", file.Name(), serviceName, imageName)
			fmt.Printf("Current tag: %s\n", imageTag)
			fmt.Printf("Human URL: %s\n", humanURLForImageName(imageName))
			tags := keepNewerTags(imageTag, tagsForImageName(imageName))
			fmt.Printf("Available tags: %v\n", tags)
		}
	}
}

func keepNewerTags(currentTag string, allTags []string) []string {
	current, err := version.NewVersion(currentTag)
	if err != nil {
		logrus.Fatal(err)
	}
	newerTags := []string{}
	for _, tag := range allTags {
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

func humanURLForImageName(imageName string) string {
	if stringStartsWith(imageName, "docker-registry.wikimedia.org") {
		return "https://" + imageName + "/tags"
	}
	if strings.Contains(imageName, "/") {
		return "https://hub.docker.com/r/" + imageName + "?tab=tags"
	}
	return "https://hub.docker.com/_/" + imageName + "?tab=tags"
}

func tagsForImageName(imageName string) []string {
	if stringStartsWith(imageName, "docker-registry.wikimedia.org") {
		v2Res := v2Response{}
		jsonFromURL("https://docker-registry.wikimedia.org/v2/"+strings.Replace(imageName, "docker-registry.wikimedia.org/", "", 1)+"/tags/list/", &v2Res)
		return v2Res.Tags
	}
	// Use v1 for docker hub, as it doesnt require authentication
	v1Res := v1Response{}
	jsonFromURL("https://registry.hub.docker.com/v1/repositories/"+imageName+"/tags", &v1Res)
	return v1Res.Tags()
}

type v1Response []struct {
	Layer string `json:"layer"`
	Name  string `json:"name"`
}

func (r v1Response) Tags() []string {
	tags := []string{}
	for i := 0; i < len(r); i++ {
		tags = append(tags, r[i].Name)
	}
	return tags
}

type v2Response struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func jsonFromURL(url string, unmarshalTo interface{}) interface{} {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logrus.Fatal(err)
	}

	req.Header.Set("User-Agent", "mwcli-tools-image-check")

	res, getErr := client.Do(req)
	if getErr != nil {
		logrus.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		logrus.Fatal(readErr)
	}

	jsonErr := json.Unmarshal(body, &unmarshalTo)
	if jsonErr != nil {
		logrus.Fatal(jsonErr)
	}

	return unmarshalTo
}

func stringStartsWith(s string, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}
