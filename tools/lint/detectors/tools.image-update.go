package detectors

import (
	"os"
	"regexp"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
	"gopkg.in/yaml.v2"
)

func DetectDataIssues() []issue.Issue {
	// Load the data yml file
	data, err := os.ReadFile("tools/image-update/data.yml")
	if err != nil {
		panic(err)
	}

	// Parse the data yml file
	var dataYml map[string]interface{}
	err = yaml.Unmarshal(data, &dataYml)
	if err != nil {
		panic(err)
	}

	issues := []issue.Issue{}

	// Check files in yml exist
	if dataYml["files"] != nil {
		dataYmlFiles := dataYml["files"].([]interface{})
		for _, file := range dataYmlFiles {
			if _, err := os.Stat(file.(string)); os.IsNotExist(err) {
				issues = append(issues, issue.Issue{
					Target:  "data.yml file: " + file.(string),
					Level:   issue.ErrorLevel,
					Code:    "data-yml-file-existence",
					Text:    "File listed in data.yml does not exist",
					Context: file.(string),
				})
			}
		}
	}

	// Check directories in yml exist
	if dataYml["directories"] != nil {
		dataYmlDirectories := dataYml["directories"].([]interface{})
		for _, directory := range dataYmlDirectories {
			if _, err := os.Stat(directory.(string)); os.IsNotExist(err) {
				issues = append(issues, issue.Issue{
					Target:  "data.yml directory: " + directory.(string),
					Level:   issue.ErrorLevel,
					Code:    "data-yml-directory-existence",
					Text:    "Directory listed in data.yml does not exist",
					Context: directory.(string),
				})
			}
		}
	}

	// Check requireRegex in images in data.yml
	dataYmlImages := dataYml["images"].([]interface{})
	for _, image := range dataYmlImages {
		imageMap := image.(map[interface{}]interface{})
		if imageMap["requireRegex"] != nil {
			_, err := regexp.Compile(imageMap["requireRegex"].(string))
			if err != nil {
				issues = append(issues, issue.Issue{
					Target:  "data.yml image requireRegex: " + imageMap["image"].(string),
					Level:   issue.ErrorLevel,
					Code:    "data-yml-image-requireRegex-compile",
					Text:    "Invalid requireRegex",
					Context: imageMap["requireRegex"].(string),
				})
			}
			// get image tag from image name
			imageTag := imageMap["image"].(string)
			imageTag = imageTag[strings.LastIndex(imageTag, ":")+1:]

			// check compiled regex matches current image tag
			if !regexp.MustCompile(imageMap["requireRegex"].(string)).MatchString(imageTag) {
				issues = append(issues, issue.Issue{
					Target:  "data.yml image requireRegex: " + imageMap["image"].(string),
					Level:   issue.ErrorLevel,
					Code:    "data-yml-image-requireRegex-match",
					Text:    "Current image tag does not match requireRegex",
					Context: imageMap["requireRegex"].(string),
				})
			}
		}
	}

	return issues
}
