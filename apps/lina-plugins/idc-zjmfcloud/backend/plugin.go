// This file declares the managed guest HTTP route group consumed by the WASM builder.

package backend

import "lina-core/pkg/plugin/pluginbridge"

// RegisterPlugin exposes only the governed customer API group to the dynamic route builder.
func RegisterPlugin(plugin pluginbridge.Declarations) error {
	return plugin.Routes().Group("/api/v1", "customer/v1")
}
