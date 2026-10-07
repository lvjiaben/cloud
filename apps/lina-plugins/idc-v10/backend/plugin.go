package backend

import "lina-core/pkg/plugin/pluginbridge"

func RegisterPlugin(plugin pluginbridge.Declarations) error {
	return plugin.Routes().Group("/api/v1", "customer/v1")
}
