package files

import (
	"os"
	"path/filepath"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/embedsync"
	"gitlab.wikimedia.org/repos/releng/cli/mount"
)

func ensureJobRunnerSitesFile(projectDirectory string) {
	mediaWikiDir := filepath.Clean(filepath.Join(projectDirectory, "mediawiki"))
	if err := os.MkdirAll(mediaWikiDir, 0o755); err != nil {
		panic(err)
	}

	jobRunnerSitesPath := filepath.Clean(filepath.Join(mediaWikiDir, "jobrunner-sites"))
	if info, err := os.Stat(jobRunnerSitesPath); err == nil && info.IsDir() {
		if removeErr := os.RemoveAll(jobRunnerSitesPath); removeErr != nil {
			panic(removeErr)
		}
	}

	file, err := os.OpenFile(jobRunnerSitesPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
}

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
			`custom(?:[-.][A-Za-z0-9_.-]+)?\.ya?ml`,
			// Custom Dockerfiles managed by `mw docker <service> image dockerfile set`
			`Dockerfile\.[A-Za-z0-9_.-]+`,
		},
	}
}

/*EnsureReady makes sure that the files component is ready.*/
func EnsureReady(projectDirectory string) {
	syncer := syncer(projectDirectory)
	syncer.EnsureFilesOnDisk()
	ensureJobRunnerSitesFile(projectDirectory)
	syncer.EnsureNoExtraFilesOnDisk()
}
