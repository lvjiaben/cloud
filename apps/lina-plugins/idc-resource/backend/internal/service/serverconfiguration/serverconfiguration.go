// Package serverconfiguration compiles frozen server choices into typed,
// secret-free resource facts for provisioning and paid configuration changes.
package serverconfiguration

import (
	"encoding/json"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// Compile validates integer sizes, maps GiB memory to MiB and distinguishes
// finite, disabled and unlimited quotas. Missing choices preserve package values.
// Invalid or missing required facts return provisionresourcecap.ErrParametersUnavailable.
func Compile(product, customer json.RawMessage) ([]resourcecap.ServerProvisionComponent, error) {
	return compile(product, customer)
}
