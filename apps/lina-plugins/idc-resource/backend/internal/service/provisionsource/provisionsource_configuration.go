// This file converts the server connector's frozen generic configuration
// objects into the resource owner's typed, secret-free provisioning facts.

package provisionsource

import (
	"bytes"
	"encoding/json"
	"io"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type frozenServerProductConfiguration struct {
	Region     string      `json:"region"`
	Line       string      `json:"line"`
	CPU        json.Number `json:"cpu"`
	Memory     json.Number `json:"memory"`
	SystemDisk json.Number `json:"system_disk"`
	DataDisk   json.Number `json:"data_disk"`
	Bandwidth  json.Number `json:"bandwidth"`
}

type frozenServerCustomerConfiguration struct {
	OperatingSystem string `json:"operating_system"`
}

// parseFrozenServerConfiguration accepts only the fields published by the
// server-owner contract. Generic product JSON stays opaque until it reaches
// this owner boundary; Provider-specific names are still resolved later by
// the selected channel's connector parameter builder.
func parseFrozenServerConfiguration(
	productConfiguration json.RawMessage,
	customerConfiguration json.RawMessage,
) ([]resourcecap.ServerProvisionComponent, error) {
	var (
		product  frozenServerProductConfiguration
		customer frozenServerCustomerConfiguration
	)
	if decodeOwnerConfiguration(productConfiguration, &product) != nil ||
		decodeOwnerConfiguration(customerConfiguration, &customer) != nil {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	cpu, ok := boundedConfigurationInteger(product.CPU, 1, 1024)
	if !ok {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	memory, ok := boundedConfigurationInteger(product.Memory, 128, 1048576)
	if !ok {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	systemDisk, ok := boundedConfigurationInteger(product.SystemDisk, 1, 1048576)
	if !ok {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	dataDisk, ok := boundedConfigurationInteger(product.DataDisk, 0, 1048576)
	if !ok {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	bandwidth, ok := boundedConfigurationInteger(product.Bandwidth, 1, 1000000)
	if !ok || product.Region == "" || product.Line == "" || customer.OperatingSystem == "" {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	components := []resourcecap.ServerProvisionComponent{
		{Type: resourcecap.ServerProvisionComponentRegion, Code: product.Region, SelectionQuantity: 1},
		{Type: resourcecap.ServerProvisionComponentLine, Code: product.Line, SelectionQuantity: 1},
		{Type: resourcecap.ServerProvisionComponentCPU, Cores: cpu, SelectionQuantity: 1},
		{Type: resourcecap.ServerProvisionComponentMemory, Megabytes: memory, SelectionQuantity: 1},
		{Type: resourcecap.ServerProvisionComponentSystemDisk, Gigabytes: systemDisk, SelectionQuantity: 1},
		{Type: resourcecap.ServerProvisionComponentBandwidth, MegabitsPerSecond: bandwidth, SelectionQuantity: 1},
		// The customer-facing operating-system choice is the connector's image
		// lookup key; no operating-system field is added to generic commerce.
		{Type: resourcecap.ServerProvisionComponentImage, Code: customer.OperatingSystem, SelectionQuantity: 1},
	}
	if dataDisk > 0 {
		components = append(components, resourcecap.ServerProvisionComponent{
			Type: resourcecap.ServerProvisionComponentDataDisk, Gigabytes: dataDisk,
			SelectionQuantity: 1,
		})
	}
	return components, nil
}

// decodeOwnerConfiguration reads only the server-owner fields. Connector
// releases may declare additional, schema-validated fields (for example an
// upstream product identifier); those fields remain opaque here and continue
// unchanged to the exact connector release that declared them.
func decodeOwnerConfiguration(payload json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return provisionresourcecap.ErrParametersUnavailable
	}
	return nil
}

func boundedConfigurationInteger(value json.Number, minimum int64, maximum int64) (int64, bool) {
	parsed, err := value.Int64()
	return parsed, err == nil && parsed >= minimum && parsed <= maximum
}
