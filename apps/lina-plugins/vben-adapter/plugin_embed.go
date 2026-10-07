package vben_adapter

import "embed"

// EmbeddedFiles contains the source-plugin manifest and independently built adapter contract.
//
//go:embed plugin.yaml frontend/dist
var EmbeddedFiles embed.FS
