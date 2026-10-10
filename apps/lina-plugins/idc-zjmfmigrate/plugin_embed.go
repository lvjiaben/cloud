// This file embeds the migration plugin manifest and delivery resources.

package main

import "embed"

// EmbeddedFiles contains only compiled delivery assets and governed resources.
//
//go:embed plugin.yaml frontend/dist manifest
var EmbeddedFiles embed.FS
