// Package idcportal embeds the website-delivery plugin resources.
package idcportal

import "embed"

// EmbeddedFiles contains the plugin manifest, governed frontend page, and runtime resources.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
