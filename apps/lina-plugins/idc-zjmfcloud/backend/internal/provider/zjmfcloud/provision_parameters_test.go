// This file verifies ZJMFCloud owns its server-plan to API-parameter mapping.

package zjmfcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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

// TestBuildProvisionDataDiskCapacity verifies the declared capacity reaches
// the final API form, while zero means no disk and invalid values fail closed.
func TestBuildProvisionDataDiskCapacity(t *testing.T) {
	t.Parallel()
	for _, capacity := range []int64{0, 100, 1048576} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			plan := zjmfProvisionTestPlan()
			plan.Components = append(plan.Components, resourcecap.ServerProvisionComponent{
				Type: resourcecap.ServerProvisionComponentDataDisk, Gigabytes: capacity, SelectionQuantity: 1,
			})
			parameters, err := resourcecap.BuildProvisionParameters(context.Background(), BuildProvisionParameters, plan)
			if err != nil {
				t.Fatal(err)
			}
			object, err := decodeParameters(parameters)
			if err != nil {
				t.Fatal(err)
			}
			form, err := provisionForm(object)
			if err != nil {
				t.Fatal(err)
			}
			if capacity == 0 {
				if form.Has("data_disk_size") {
					t.Fatal("zero capacity created an upstream disk parameter")
				}
			} else if form.Get("data_disk_size") != fmt.Sprint(capacity) {
				t.Fatalf("capacity was lost: %v", form)
			}
		})
	}
	for _, value := range []string{"-1", "1.5", "1048577", "01", "9223372036854775808"} {
		object, err := decodeParameters(json.RawMessage(`{"cpu":2,"memory":4096,"os":3,"client":88,"hostname":"srv.test","data_disk_size":` + strconv.Quote(value) + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = provisionForm(object); err == nil {
			t.Fatalf("invalid disk capacity accepted: %s", value)
		}
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

// TestProvisionBackupQuotaSemantics proves disabled and unlimited remain
// distinct through the final supplier form, including invalid combinations.
func TestProvisionBackupQuotaSemantics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		count     int64
		unlimited bool
		want      string
	}{
		{"disabled", 0, false, "-1"},
		{"unlimited", 0, true, "0"},
		{"limited", 5, false, "5"},
		{"conflicting", 5, true, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := zjmfProvisionTestPlan()
			plan.Components = append(plan.Components, resourcecap.ServerProvisionComponent{
				Type: resourcecap.ServerProvisionComponentBackup, Count: test.count,
				Unlimited: test.unlimited, SelectionQuantity: 1,
			})
			parameters, err := resourcecap.BuildProvisionParameters(context.Background(), BuildProvisionParameters, plan)
			if test.want == "" {
				if err == nil {
					t.Fatal("conflicting quota accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			object, err := decodeParameters(parameters)
			if err != nil {
				t.Fatal(err)
			}
			form, err := provisionForm(object)
			if err != nil || form.Get("backup_num") != test.want {
				t.Fatalf("backup quota lost: %v %v", form, err)
			}
		})
	}
	for _, key := range []string{"backup_num", "snap_num", "nat_acl_limit", "nat_web_limit"} {
		for _, value := range []string{"-1", "0", "5", "-2", "00", "1.5", "9223372036854775808", ""} {
			object, err := decodeParameters(json.RawMessage(`{"cpu":2,"memory":4096,"os":3,"client":88,"hostname":"srv.test",` + strconv.Quote(key) + `:` + strconv.Quote(value) + `}`))
			if err != nil {
				t.Fatal(err)
			}
			_, err = provisionForm(object)
			valid := value == "-1" || value == "0" || value == "5"
			if (err == nil) != valid {
				t.Fatalf("%s=%q validity: %v", key, value, err)
			}
		}
	}
}

// TestProvisionAccessoryQuotaSemantics verifies the additional quota kinds
// survive the owner contract and final form without implicit supplier defaults.
func TestProvisionAccessoryQuotaSemantics(t *testing.T) {
	t.Parallel()
	for kind, key := range map[resourcecap.ServerProvisionComponentType]string{
		resourcecap.ServerProvisionComponentSnapshot:   "snap_num",
		resourcecap.ServerProvisionComponentNATForward: "nat_acl_limit",
		resourcecap.ServerProvisionComponentNATWebsite: "nat_web_limit",
	} {
		t.Run(string(kind), func(t *testing.T) {
			for _, test := range []struct {
				count     int64
				unlimited bool
				quantity  int64
				want      string
			}{
				{0, false, 1, "-1"}, {0, true, 1, "0"}, {5, false, 1, "5"},
				{5, true, 1, ""}, {-1, false, 1, ""}, {5, false, 2, ""},
			} {
				plan := zjmfProvisionTestPlan()
				component := resourcecap.ServerProvisionComponent{Type: kind, Count: test.count, Unlimited: test.unlimited, SelectionQuantity: test.quantity}
				plan.Components = append(plan.Components, component)
				parameters, err := resourcecap.BuildProvisionParameters(context.Background(), BuildProvisionParameters, plan)
				if test.want == "" {
					if err == nil {
						t.Fatalf("invalid quota accepted: %#v", component)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				object, err := decodeParameters(parameters)
				if err != nil {
					t.Fatal(err)
				}
				form, err := provisionForm(object)
				if err != nil || form.Get(key) != test.want {
					t.Fatalf("quota lost: %s=%q want=%q err=%v", key, form.Get(key), test.want, err)
				}
				// Duplicate facts must never silently override a purchased quota.
				plan.Components = append(plan.Components, component)
				if _, err = resourcecap.BuildProvisionParameters(context.Background(), BuildProvisionParameters, plan); err == nil {
					t.Fatal("duplicate quota accepted")
				}
			}
		})
	}
	parameters, err := resourcecap.BuildProvisionParameters(context.Background(), BuildProvisionParameters, zjmfProvisionTestPlan())
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err = json.Unmarshal(parameters, &object); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"snap_num", "nat_acl_limit", "nat_web_limit"} {
		if _, exists := object[key]; exists {
			t.Fatalf("legacy plan gained implicit quota %s", key)
		}
	}
}
