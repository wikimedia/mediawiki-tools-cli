package files

import (
	"embed"

	"gitlab.wikimedia.org/releng/cli/internal/util/embedsync"
)

//go:embed embed
var content embed.FS

func syncer(projectDirectory string) embedsync.EmbeddingDiskSync {
	return embedsync.EmbeddingDiskSync{
		Embed:     content,
		EmbedPath: "embed",
		DiskPath:  projectDirectory,
		IgnoreFiles: []string{
			// Used by docker-compose to store currnet environment variables in
			".env",
			// Used by the dev environment to store hosts that need adding to the hosts file
			"record-hosts",
			// Used by folks that want to define a custom set of docker-compose services
			"custom.yml",
		},
	}
}

/*EnsureReady makes sure that the files component is ready.*/
func EnsureReady(projectDirectory string) {
	syncer := syncer(projectDirectory)
	syncer.EnsureFilesOnDisk()
	syncer.EnsureNoExtraFilesOnDisk()
}
