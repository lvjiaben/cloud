// This file embeds the compliance plugin manifest and delivery resources.

package idccompliance

import "embed"

// EmbeddedFiles contains the plugin manifest, frontend scaffold, and manifest resources.
//
//go:embed plugin.yaml frontend/pages manifest
var EmbeddedFiles embed.FS
