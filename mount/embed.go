package mount

import "embed"

const DevEmbedPath = "dev"

//go:embed dev
var DevContent embed.FS
