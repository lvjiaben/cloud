// This file verifies purchased sizing and quota semantics at the shared owner compiler.

package serverconfiguration

import (
	"encoding/json"
	"errors"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
)

// TestPurchasedSizingReachesProvisionPlan proves purchase values replace fixed
// defaults, including explicit zero disks, while GiB is converted to MiB once.
func TestPurchasedSizingReachesProvisionPlan(t *testing.T) {
	t.Parallel()
	components, err := Compile(
		json.RawMessage(`{"region":"1","line":"2","cpu":2,"memory":4096,"system_disk":40,"data_disk":100,"bandwidth":100}`),
		json.RawMessage(`{"operating_system":"3","cpu":4,"memory_gib":8,"system_disk":80,"data_disk":0,"bandwidth":200,"snapshot_count":0,"backup_count":5,"nat_forward_unlimited":"unlimited","nat_website_count":0}`))
	if err != nil {
		t.Fatal(err)
	}
	byType := make(map[resourcecap.ServerProvisionComponentType]resourcecap.ServerProvisionComponent)
	for _, component := range components {
		byType[component.Type] = component
	}
	if byType[resourcecap.ServerProvisionComponentCPU].Cores != 4 || byType[resourcecap.ServerProvisionComponentMemory].Megabytes != 8192 || byType[resourcecap.ServerProvisionComponentSystemDisk].Gigabytes != 80 || byType[resourcecap.ServerProvisionComponentBandwidth].MegabitsPerSecond != 200 {
		t.Fatalf("purchased sizes lost: %+v", components)
	}
	if _, exists := byType[resourcecap.ServerProvisionComponentDataDisk]; exists {
		t.Fatal("zero purchase retained the package data disk")
	}
	if byType[resourcecap.ServerProvisionComponentSnapshot].Unlimited || byType[resourcecap.ServerProvisionComponentSnapshot].Count != 0 || byType[resourcecap.ServerProvisionComponentBackup].Count != 5 || !byType[resourcecap.ServerProvisionComponentNATForward].Unlimited || byType[resourcecap.ServerProvisionComponentNATWebsite].Unlimited {
		t.Fatalf("quota meanings lost: %+v", components)
	}
	if len(components) != 11 {
		t.Fatalf("missing explicit disabled quotas: %+v", components)
	}
}

func TestPurchasedSizingRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, selection := range []string{
		`"cpu":0`, `"cpu":1.5`, `"memory_gib":1025`, `"data_disk":-1`,
		`"snapshot_count":-1`, `"backup_count":1.5`,
		`"nat_forward_unlimited":"unknown"`,
		`"snapshot_unlimited":"unlimited","snapshot_count":2`,
	} {
		t.Run(selection, func(t *testing.T) {
			components, err := Compile(json.RawMessage(`{"region":"1","line":"2","cpu":2,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100}`), json.RawMessage(`{"operating_system":"3",`+selection+`}`))
			if !errors.Is(err, provisionresourcecap.ErrParametersUnavailable) || components != nil {
				t.Fatalf("invalid purchase accepted: %+v %v", components, err)
			}
		})
	}
}
