// Package backend declares the managed migration plugin's dynamic HTTP routes.
package backend

import "lina-core/pkg/plugin/pluginbridge"

// RegisterPlugin exposes the administrator routes to the WASM artifact builder.
func RegisterPlugin(plugin pluginbridge.Declarations) error {
	return plugin.Routes().Group("/api/v1", "admin/v1")
}
