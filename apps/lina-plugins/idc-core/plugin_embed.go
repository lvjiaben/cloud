// This file embeds the IDC core plugin manifest, frontend scaffold, and manifest resources.

package idccore

import "embed"

// EmbeddedFiles contains the plugin manifest, frontend scaffold, and manifest resources.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
