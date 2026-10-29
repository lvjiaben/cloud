// This file embeds the resource plugin's admin and customer pages with its manifest resources.

package idcresource

import "embed"

// EmbeddedFiles includes the built customer page used by official source-plugin binaries.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
