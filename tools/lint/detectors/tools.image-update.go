package detectors

import (
	"os"

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

	// Check directories in yml exist
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

	return issues
}
