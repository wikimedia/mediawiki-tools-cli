package embedsync

func syncer(projectDirectory string) EmbeddingDiskSync {
	return EmbeddingDiskSync{
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
