// This file embeds the AI plugin manifest and delivery resources.

package idcai

import "embed"

// EmbeddedFiles contains the plugin manifest, frontend scaffold, and manifest resources.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
