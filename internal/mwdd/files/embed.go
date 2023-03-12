package files

import (
	"embed"
	"os"
	"path/filepath"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/embedsync"
)

//go:embed embed
var content embed.FS

func syncer(projectDirectory string) embedsync.EmbeddingDiskSync {
	return embedsync.EmbeddingDiskSync{
		Embed:     content,
		EmbedPath: "embed",
		DiskPath:  projectDirectory,
		IgnoreFiles: []string{
			// Used by docker-compose to store current environment variables in
			`\.env`,
			// Used by the dev environment to store hosts that need adding to the hosts file
			`record\-hosts`,
			// Used by folks that want to define a custom set of docker-compose services
			`custom\.yml`,
			`custom-\w+\.yml`,
		},
	}
}

/*EnsureReady makes sure that the files component is ready.*/
func EnsureReady(projectDirectory string) {
	syncer := syncer(projectDirectory)
	syncer.EnsureFilesOnDisk()
	syncer.EnsureNoExtraFilesOnDisk()
}

/*ListRawDcYamlFilesInContextOfProjectDirectory ...*/
func ListRawDcYamlFilesInContextOfProjectDirectory(projectDirectory string) []string {
	// TODO this function should live in the mwdd struct?
	var files []string

	for _, file := range listRawFiles(projectDirectory) {
		if filepath.Ext(file) == ".yml" {
			files = append(files, filepath.Base(file))
		}
	}

	return files
}

/*listRawFiles lists the raw docker-compose file paths that are currently on disk.*/
func listRawFiles(projectDirectory string) []string {
	var files []string

	err := filepath.Walk(projectDirectory, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}
