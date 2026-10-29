// Package plugin_manager embeds the built-in plugin management workspace.
package plugin_manager

import "embed"

// EmbeddedFiles supplies the manifest, configuration, and source pages.
//
//go:embed plugin.yaml manifest frontend/pages
var EmbeddedFiles embed.FS
