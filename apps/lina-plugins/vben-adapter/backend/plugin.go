// Package backend registers the frontend-only Vben adapter source plugin.
package backend

import (
	"lina-core/pkg/plugin/pluginhost"
	vbenadapterplugin "lina-plugin-vben-adapter"
)

const pluginID = "vben-adapter"

func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(vbenadapterplugin.EmbeddedFiles)
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}
