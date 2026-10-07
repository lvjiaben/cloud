// Package sharedservices embeds the shared-services plugin resources.
package sharedservices

import "embed"

// EmbeddedFiles contains the plugin manifest and installation resources.
//
//go:embed plugin.yaml manifest
var EmbeddedFiles embed.FS
