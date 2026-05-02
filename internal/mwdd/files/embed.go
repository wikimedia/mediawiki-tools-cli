package files

import (
	"gitlab.wikimedia.org/repos/releng/cli/mount"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/embedsync"
)

func syncer(projectDirectory string) embedsync.EmbeddingDiskSync {
	return embedsync.EmbeddingDiskSync{
		Embed:     mount.DevContent,
		EmbedPath: mount.DevEmbedPath,
		DiskPath:  projectDirectory,
		IgnoreFiles: []string{
			// Used by docker compose to store current environment variables in
			`\.env`,
			// Used by the dev environment to store hosts that need adding to the hosts file
			`record\-hosts`,
			// Used by the dev environment to store the list of sites to run mediawiki-jobrunner against
			`mediawiki\/jobrunner\-sites`,
			// Used by folks that want to define a custom set of docker compose services
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
