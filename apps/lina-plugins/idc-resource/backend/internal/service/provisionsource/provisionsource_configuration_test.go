// This file verifies bounded server-owner parsing of frozen generic product
// and customer configuration snapshots while connector-owned fields remain
// opaque to the resource owner.

package provisionsource

import (
	"encoding/json"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestParseFrozenServerConfigurationBuildsOwnerFacts(t *testing.T) {
	t.Parallel()
	components, err := parseFrozenServerConfiguration(
		json.RawMessage(`{"region":"cn-south","line":"premium","cpu":2,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100}`),
		json.RawMessage(`{"operating_system":"ubuntu-2204"}`),
	)
	if err != nil {
		t.Fatalf("parseFrozenServerConfiguration() error = %v", err)
	}
	if len(components) != 7 || components[0].Type != resourcecap.ServerProvisionComponentRegion ||
		components[2].Cores != 2 || components[3].Megabytes != 4096 ||
		components[4].Gigabytes != 40 || components[5].MegabitsPerSecond != 100 ||
		components[6].Type != resourcecap.ServerProvisionComponentImage ||
		components[6].Code != "ubuntu-2204" {
		t.Fatalf("unexpected server-owner components: %#v", components)
	}
}

func TestParseFrozenServerConfigurationKeepsPositiveDataDisk(t *testing.T) {
	t.Parallel()
	components, err := parseFrozenServerConfiguration(
		json.RawMessage(`{"region":"cn-south","line":"premium","cpu":2,"memory":4096,"system_disk":40,"data_disk":100,"bandwidth":100}`),
		json.RawMessage(`{"operating_system":"ubuntu-2204"}`),
	)
	if err != nil || len(components) != 8 ||
		components[7].Type != resourcecap.ServerProvisionComponentDataDisk ||
		components[7].Gigabytes != 100 {
		t.Fatalf("positive data disk components=%#v err=%v", components, err)
	}
}

func TestParseFrozenServerConfigurationAcceptsConnectorOwnedFields(t *testing.T) {
	t.Parallel()
	components, err := parseFrozenServerConfiguration(
		json.RawMessage(`{"region":"cn-south","line":"premium","cpu":2,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100,"upstream_product_id":"81","upstream_region_option_id":"1001"}`),
		json.RawMessage(`{"operating_system":"ubuntu-2204","upstream_validation_profile":"default"}`),
	)
	if err != nil || len(components) != 7 {
		t.Fatalf("connector-owned configuration was not kept opaque: components=%#v err=%v", components, err)
	}
}

func TestParseFrozenServerConfigurationRejectsMissingAndFractionalValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		product  string
		customer string
	}{
		{name: "missing line", product: `{"region":"cn-south","cpu":2,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100}`, customer: `{"operating_system":"ubuntu-2204"}`},
		{name: "fractional cpu", product: `{"region":"cn-south","line":"premium","cpu":2.5,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100}`, customer: `{"operating_system":"ubuntu-2204"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseFrozenServerConfiguration(
				json.RawMessage(test.product), json.RawMessage(test.customer),
			)
			if !errors.Is(err, provisionresourcecap.ErrParametersUnavailable) {
				t.Fatalf("parseFrozenServerConfiguration() error = %v", err)
			}
		})
	}
}
