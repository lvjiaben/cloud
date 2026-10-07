// This file embeds the IDC resource plugin manifest, frontend scaffold, and manifest resources.

package idcresource

import "embed"

// EmbeddedFiles contains the plugin manifest, frontend scaffold, and manifest resources.
//
//go:embed plugin.yaml frontend manifest
var EmbeddedFiles embed.FS
