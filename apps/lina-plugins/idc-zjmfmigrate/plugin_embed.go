// This file embeds the migration plugin manifest and delivery resources.

package main

import "embed"

// EmbeddedFiles contains the plugin manifest, frontend scaffold, and manifest resources.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
