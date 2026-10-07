// This file embeds the notification plugin manifest and delivery resources.

package idcnotify

import "embed"

// EmbeddedFiles contains the plugin manifest, plugin-owned frontend, and manifest resources.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
