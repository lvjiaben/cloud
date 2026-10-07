// This file verifies ZJMFCloud owns its server-plan to API-parameter mapping.

package zjmfcloud

import (
	"context"
	"encoding/json"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// TestBuildProvisionParametersUsesServerPluginFacts verifies the resource
// owner no longer needs to understand ZJMFCloud parameter names.
func TestBuildProvisionParametersUsesServerPluginFacts(t *testing.T) {
	t.Parallel()
	plan := zjmfProvisionTestPlan()
	parameters, err := resourcecap.BuildProvisionParameters(
		context.Background(), BuildProvisionParameters, plan,
	)
	if err != nil {
		t.Fatalf("BuildProvisionParameters() error = %v", err)
	}
	var values map[string]any
	if err = json.Unmarshal(parameters, &values); err != nil {
		t.Fatalf("decode parameters: %v", err)
	}
	for key, expected := range map[string]any{
		"area": "1", "node": "2", "os": "3", "client": "88",
		"hostname": plan.Hostname, "cpu": float64(2), "memory": float64(4096),
		"ip_num": float64(1), "in_bw": float64(100), "out_bw": float64(100),
		"traffic_quota": float64(1000),
	} {
		if values[key] != expected {
			t.Fatalf("parameter %s=%v want=%v; all=%s", key, values[key], expected, parameters)
		}
	}
}

// TestBuildProvisionParametersRejectsUnsupportedServerFacts verifies an
// unsupported component fails before any outbound Provider request exists.
func TestBuildProvisionParametersRejectsUnsupportedServerFacts(t *testing.T) {
	t.Parallel()
	plan := zjmfProvisionTestPlan()
	plan.Components = append(plan.Components, resourcecap.ServerProvisionComponent{
		Type: resourcecap.ServerProvisionComponentDataDisk, Gigabytes: 100,
		Media: "ssd", SelectionQuantity: 1,
	})
	if _, err := resourcecap.BuildProvisionParameters(
		context.Background(), BuildProvisionParameters, plan,
	); err == nil {
		t.Fatal("unsupported data disk was accepted")
	}
}

func zjmfProvisionTestPlan() resourcecap.ServerProvisionPlan {
	return resourcecap.ServerProvisionPlan{
		ExternalPrincipalRef: "88",
		Hostname:             "server-1.example.test",
		Selector: json.RawMessage(
			`{"provision":{"provider":"zjmfcloud-v1","areas":{"cn-south":"1"},"nodes":{"premium":"2"},"node_groups":{},"images":{"ubuntu-2204":"3"},"defaults":{"network_type":"normal","ip_group":"4","traffic_type":3,"node_priority":1}}}`,
		),
		Components: []resourcecap.ServerProvisionComponent{
			{Type: resourcecap.ServerProvisionComponentRegion, Code: "cn-south", SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentLine, Code: "premium", SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentCPU, Cores: 2, SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentMemory, Megabytes: 4096, SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentImage, Code: "ubuntu-2204", SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentIP, Count: 1, Family: "ipv4", SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentBandwidth, MegabitsPerSecond: 100, SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentTraffic, Gigabytes: 1000, SelectionQuantity: 1},
		},
	}
}
