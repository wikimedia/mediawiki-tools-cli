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

	// TODO test rgeex of sameTagMatcher in image groups...

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
	allImages := []interface{}{}

	// Collect images form imageGroups
	if dataYml["imageGroups"] != nil {
		dataYmlImageGroups := dataYml["imageGroups"].([]interface{})
		for _, imageGroup := range dataYmlImageGroups {
			imageGroupMap := imageGroup.(map[interface{}]interface{})
			if imageGroupMap["images"] != nil {
				imageGroupImages := imageGroupMap["images"].([]interface{})
				allImages = append(allImages, imageGroupImages...)
			}
		}
	}
	// And collect normal images
	if dataYml["images"] != nil {
		allImages = append(allImages, dataYml["images"].([]interface{})...)
	}
	// And make sure the regexes look correct
	dataYmlImages := allImages
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
				continue
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
