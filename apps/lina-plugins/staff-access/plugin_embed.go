package staff_access

import "embed"

// EmbeddedFiles contains the source-plugin manifest, SQL and frontend assets.
//
//go:embed plugin.yaml manifest frontend/dist
var EmbeddedFiles embed.FS
