// This file delegates frozen server sizing to the resource-owned compiler shared
// by provisioning and paid configuration changes. It never interprets secrets.

package provisionsource

import (
	"encoding/json"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverconfiguration"
)

func parseFrozenServerConfiguration(product, customer json.RawMessage) ([]resourcecap.ServerProvisionComponent, error) {
	return serverconfiguration.Compile(product, customer)
}
