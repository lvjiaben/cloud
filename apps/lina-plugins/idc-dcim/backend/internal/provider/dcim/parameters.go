// This file maps the generic server plan to the proven BMS creation form.

package dcim

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type productConfiguration struct {
	ServerGroup    string `json:"server_group"`
	IPAddressCount int64  `json:"ip_address_count"`
	IPAddressGroup string `json:"ip_address_group,omitempty"`
	StoragePool    string `json:"storage_pool,omitempty"`
	SnapshotCount  int64  `json:"snapshot_count,omitempty"`
	BackupCount    int64  `json:"backup_count,omitempty"`
}

type customerConfiguration struct {
	OperatingSystem string `json:"operating_system"`
}

type bmsProvisionParameters struct {
	Name                 string `json:"name"`
	Type                 int    `json:"type"`
	Size                 int64  `json:"size"`
	ExternalPrincipalRef string `json:"name3"`
	GroupID              string `json:"group_id"`
	OSID                 string `json:"os_id"`
	IPCount              int64  `json:"ip_num"`
	IPGroupID            string `json:"ip_group_id,omitempty"`
	DataSize             int64  `json:"data_size"`
	StoragePoolID        string `json:"ceph_id,omitempty"`
	BackupCount          int64  `json:"backup_num"`
	SnapshotCount        int64  `json:"snap_num"`
}

func buildProvisionParameters(protocol Protocol, plan resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
	if protocol != ProtocolBMS {
		return nil, errUnsupported
	}
	if err := plan.Validate(); err != nil {
		return nil, errInvalidConfiguration
	}
	configuration, err := decodeProductConfiguration(plan.ProductConfiguration)
	if err != nil {
		return nil, err
	}
	customer, err := decodeCustomerConfiguration(plan.CustomerConfiguration)
	if err != nil {
		return nil, err
	}
	parameters := bmsProvisionParameters{
		Name: strings.TrimSpace(plan.Hostname), Type: 2,
		ExternalPrincipalRef: plan.ExternalPrincipalRef, GroupID: configuration.ServerGroup,
		OSID: customer.OperatingSystem, IPCount: configuration.IPAddressCount,
		IPGroupID: configuration.IPAddressGroup, StoragePoolID: configuration.StoragePool,
		BackupCount: configuration.BackupCount, SnapshotCount: configuration.SnapshotCount,
	}
	for _, component := range plan.Components {
		switch component.Type {
		case resourcecap.ServerProvisionComponentSystemDisk:
			parameters.Size += component.Gigabytes * component.SelectionQuantity
		case resourcecap.ServerProvisionComponentDataDisk:
			parameters.DataSize += component.Gigabytes * component.SelectionQuantity
		}
	}
	if parameters.Name == "" || len(parameters.Name) > 255 || parameters.Size < 1 {
		return nil, errInvalidConfiguration
	}
	return json.Marshal(parameters)
}

func decodeProductConfiguration(raw json.RawMessage) (productConfiguration, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var configuration productConfiguration
	if decoder.Decode(&configuration) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		!positiveDecimal(configuration.ServerGroup) || configuration.IPAddressCount < 1 || configuration.IPAddressCount > 256 ||
		configuration.BackupCount < 0 || configuration.SnapshotCount < 0 || configuration.BackupCount > 1024 || configuration.SnapshotCount > 1024 {
		return productConfiguration{}, errInvalidConfiguration
	}
	for _, optional := range []string{configuration.IPAddressGroup, configuration.StoragePool} {
		if optional != "" && !positiveDecimal(optional) {
			return productConfiguration{}, errInvalidConfiguration
		}
	}
	return configuration, nil
}

func decodeCustomerConfiguration(raw json.RawMessage) (customerConfiguration, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var configuration customerConfiguration
	if decoder.Decode(&configuration) != nil || decoder.Decode(&struct{}{}) != io.EOF || !positiveDecimal(configuration.OperatingSystem) {
		return customerConfiguration{}, errInvalidConfiguration
	}
	return configuration, nil
}

func positiveDecimal(value string) bool {
	if value == "" || len(value) > 20 || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "0") && value != "0" {
		return false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed > 0
}
