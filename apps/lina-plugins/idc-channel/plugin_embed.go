// This file embeds the downstream-channel plugin manifest and delivery resources.

package idcchannel

import "embed"

// EmbeddedFiles contains the plugin manifest, frontend scaffold, and manifest resources.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
