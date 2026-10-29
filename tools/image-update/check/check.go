package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
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
	Failed    []ImageData
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

		tags, err := tagsForImageName(imageName)
		if err != nil {
			logrus.Errorf("Could not check %s: %v", image, err)
			result.Failed = append(result.Failed, imageData)
			continue
		}
		tagsOfInterest := keepTagsMatchingRegex(keepNewerTags(imageTag, tags), imageData.RequireRegex)

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

			tags, err := tagsForImageName(imageName)
			if err != nil {
				logrus.Errorf("Could not check %s: %v", image, err)
				result.Failed = append(result.Failed, imageData)
				continue
			}
			tagsOfInterest := keepTagsMatchingRegex(keepNewerTags(imageTag, tags), imageData.RequireRegex)

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
		if len(imageUpdatePairs) > 0 && (len(imageUpdatePairs) != len(imageGroup.Images)*2 || !oneNewtag) {
			logrus.Errorf("No complete matching build for image group %s; leaving the group unchanged", imageGroup.Name)
			result.Failed = append(result.Failed, imageGroup.Images...)
			continue
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
				fmt.Printf("Require Regex: %s\n", v.RequireRegex)
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

	{
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

	{
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
		fmt.Println("A snippet for use in Gitlab CI is written to tools/image-update/.gitlab.update.yaml")
	}
	if len(result.Failed) > 0 {
		fmt.Println("The following images could not be checked:")
		for _, image := range result.Failed {
			fmt.Println(image.Image)
		}
		os.Exit(1)
	}
}

func (i ImageData) nameAndTagFromTaggedName() (string, string) {
	if imageName, _, ok := strings.Cut(i.Image, "@sha256:"); ok {
		return imageName, ""
	}
	r := regexp.MustCompile(`(\$\{.*:\-)?([\w\/\-\.]+):([\w\-\_\.]+)(\})?`)
	regexSplit := r.FindStringSubmatch(i.Image)
	imageName := regexSplit[2]
	imageTag := regexSplit[3]
	return imageName, imageTag
}

func (i ImageData) humanURLForImageName() string {
	imageName, _ := i.nameAndTagFromTaggedName()
	if strings.HasPrefix(imageName, "ghcr.io/") || strings.HasPrefix(imageName, "registry.gitlab.com/") {
		return "https://" + imageName
	}
	if stringStartsWith(imageName, "docker-registry.wikimedia.org") {
		return "https://" + imageName + "/tags"
	}
	if strings.HasPrefix(imageName, "quay.io/") {
		return "https://" + imageName + "?tab=tags"
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

	sort.SliceStable(newerTags, func(i, j int) bool {
		left, _ := version.NewVersion(newerTags[i])
		right, _ := version.NewVersion(newerTags[j])
		return left.LessThan(right)
	})
	return newerTags
}

func keepTagsMatchingRegex(tags []string, regex string) []string {
	if regex == "" {
		return tags
	}
	matchingTags := []string{}
	for _, tag := range tags {
		if stringMatchesRegex(tag, "^(?:"+regex+")$") {
			matchingTags = append(matchingTags, tag)
		}
	}
	return matchingTags
}

func stringMatchesRegex(str string, regex string) bool {
	r := regexp.MustCompile(regex)
	return r.MatchString(str)
}

var (
	registryClient = &http.Client{Timeout: 20 * time.Second}
	tagCache       = map[string][]string{}
)

func tagsForImageName(imageName string) ([]string, error) {
	if tags, ok := tagCache[imageName]; ok {
		return tags, nil
	}
	var tags []string
	var err error
	switch {
	case strings.HasPrefix(imageName, "docker-registry.wikimedia.org/"), strings.HasPrefix(imageName, "quay.io/"):
		registry, repository, _ := strings.Cut(imageName, "/")
		tags, err = registryTags("https://" + registry + "/v2/" + repository + "/tags/list")
	default:
		repository := strings.TrimPrefix(strings.TrimPrefix(imageName, "docker.io/"), "index.docker.io/")
		if !strings.Contains(repository, "/") {
			repository = "library/" + repository
		}
		tags, err = dockerHubTags(repository)
	}
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("registry returned no tags for %s", imageName)
	}
	tagCache[imageName] = tags
	return tags, nil
}

func registryTags(apiURL string, token ...string) ([]string, error) {
	var tags []string
	seen := map[string]bool{}
	for apiURL != "" {
		if seen[apiURL] {
			return nil, fmt.Errorf("registry pagination loop at %s", apiURL)
		}
		seen[apiURL] = true
		var page struct {
			Tags []string `json:"tags"`
		}
		headers, err := jsonFromURL(apiURL, &page, token...)
		if err != nil {
			return nil, err
		}
		tags = append(tags, page.Tags...)
		nextURL := ""
		for _, link := range strings.Split(headers.Get("Link"), ",") {
			if strings.Contains(link, `rel="next"`) {
				start, end := strings.Index(link, "<"), strings.Index(link, ">")
				if start >= 0 && end > start {
					req, err := http.NewRequest(http.MethodGet, apiURL, nil)
					if err != nil {
						return nil, err
					}
					next, err := req.URL.Parse(link[start+1 : end])
					if err != nil {
						return nil, err
					}
					nextURL = next.String()
				}
			}
		}
		apiURL = nextURL
	}
	return tags, nil
}

func dockerHubTags(repository string) ([]string, error) {
	var auth struct {
		Token string `json:"token"`
	}
	if _, err := jsonFromURL("https://auth.docker.io/token?service=registry.docker.io&scope=repository:"+repository+":pull", &auth); err != nil {
		return nil, err
	}
	if auth.Token == "" {
		return nil, fmt.Errorf("Docker Hub returned no anonymous pull token")
	}
	return registryTags("https://registry-1.docker.io/v2/"+repository+"/tags/list?n=10000", auth.Token)
}

func jsonFromURL(url string, unmarshalTo interface{}, token ...string) (http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", cli.HTTPUserAgent)
	if len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token[0])
	}
	res, err := registryClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %s", url, res.Status)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(unmarshalTo); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return res.Header, nil
}

func stringStartsWith(s string, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}
