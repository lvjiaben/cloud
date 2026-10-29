// These tests verify actual v1 field mapping, bounded validation, deterministic
// VPC isolation and the managed confidential transport after data-disk translation.
package zjmfcloud

import (
	"context"
	"encoding/json"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
)

func TestAdvancedProvisionFieldsReachActualAdministratorForm(t *testing.T) {
	plan := zjmfProvisionTestPlan()
	plan.ProductConfiguration = json.RawMessage(`{"network_type":"normal","port":"auto","system_read_mbps":500,"system_write_mbps":0,"data_read_iops":2000}`)
	plan.CustomerConfiguration = json.RawMessage(`{"network_type":"vpc","port":"22000","system_read_mbps":250}`)
	parameters, err := BuildProvisionParameters(context.Background(), plan)
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
	for key, want := range map[string]string{"network_type": "vpc", "port": "22000", "system_read_bytes_sec": "250", "system_write_bytes_sec": "0", "data_read_iops_sec": "2000"} {
		if form.Get(key) != want {
			t.Fatalf("v1 field %s=%s want %s", key, form.Get(key), want)
		}
	}
	original := form.Get("vpc_name")
	if original == "" {
		t.Fatal("private network lacks isolated identity")
	}
	again, err := BuildProvisionParameters(context.Background(), plan)
	if err != nil || string(again) != string(parameters) {
		t.Fatal("same frozen plan changed")
	}
	plan.Hostname = "another-server.example.test"
	changed, err := BuildProvisionParameters(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	changedObject, err := decodeParameters(changed)
	if err != nil {
		t.Fatal(err)
	}
	changedForm, err := provisionForm(changedObject)
	if err != nil || changedForm.Get("vpc_name") == original {
		t.Fatal("another cloud reused a network")
	}
}

func TestAdvancedProvisionRejectsInvalidValuesBeforePayment(t *testing.T) {
	for _, raw := range []string{`{"network_type":"guess"}`, `{"port":"0"}`, `{"port":"65536"}`, `{"port":"022"}`, `{"port":"Auto"}`, `{"port":"a&b"}`, `{"system_read_mbps":-1}`, `{"data_write_iops":1.5}`, `{"system_write_iops":2147483648}`, `{"data_read_mbps":"50"}`} {
		if ValidateOptionalProvisionConfiguration(json.RawMessage(raw)) == nil {
			t.Fatalf("invalid value accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{}`, `{"network_type":"normal","port":"auto"}`, `{"port":"65535","system_read_mbps":0}`, `{"port":""}`} {
		if err := ValidateOptionalProvisionConfiguration(json.RawMessage(raw)); err != nil {
			t.Fatalf("valid configuration rejected %s: %v", raw, err)
		}
	}
}

func TestManagedProvisionUsesNestedDataDiskAndAdvancedForm(t *testing.T) {
	plan := zjmfProvisionTestPlan()
	plan.Components = append(plan.Components, resourcecap.ServerProvisionComponent{Type: resourcecap.ServerProvisionComponentDataDisk, Gigabytes: 100, SelectionQuantity: 1})
	plan.ProductConfiguration = json.RawMessage(`{"port":"auto","system_read_mbps":500}`)
	parameters, err := BuildProvisionParameters(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(resourcecap.DynamicOperationRequest{RequestID: "managed-advanced-1", Capability: resourcecap.CapabilityProvision, Parameters: parameters, ConfidentialInputNames: []string{"password"}})
	if err != nil {
		t.Fatal(err)
	}
	executor := &lifecycleExecutor{t: t, response: `{"id":41,"taskid":71}`}
	if _, err = ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodExecute, payload); err != nil {
		t.Fatal(err)
	}
	if len(executor.calls) != 2 {
		t.Fatal("unexpected managed write count")
	}
	form := executor.calls[1].Form
	if form["other_data_disk[0][size]"] != "100" || form["data_disk_size"] != "" || form["system_read_bytes_sec"] != "500" || form["port"] != "auto" {
		t.Fatalf("managed v1 mapping lost: %+v", form)
	}
}

func TestFinancePriorityTrafficCycleAndDiskProfilesReachActualForm(t *testing.T) {
	plan := zjmfProvisionTestPlan()
	plan.ProductConfiguration = json.RawMessage(`{"node_priority":"3","traffic_bill_type":"last_30days","system_disk_io_limit":"500,500,2000,2000","data_disk_io_limit":"0,0,0,0"}`)
	parameters, err := BuildProvisionParameters(context.Background(), plan)
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
	if form.Has("traffic_bill_type") || form.Get("reset_flow_day") == "" {
		t.Fatal("finance billing policy was not translated into native reset day")
	}
	for key, want := range map[string]string{"node_priority": "3", "system_read_bytes_sec": "500", "system_write_bytes_sec": "500", "system_read_iops_sec": "2000", "system_write_iops_sec": "2000", "data_read_bytes_sec": "0", "data_write_iops_sec": "0"} {
		if form.Get(key) != want {
			t.Fatalf("%s=%s want %s", key, form.Get(key), want)
		}
	}
	for _, invalid := range []string{`{"node_priority":"0"}`, `{"node_priority":"5"}`, `{"traffic_bill_type":"30"}`, `{"system_disk_io_limit":"500,500,2000"}`, `{"system_disk_io_limit":"-1,0,0,0"}`, `{"data_disk_io_limit":"1,2,3,2147483648"}`, `{"system_disk_io_limit":"500,500,2000,2000","system_read_mbps":100}`} {
		if ValidateOptionalProvisionConfiguration(json.RawMessage(invalid)) == nil {
			t.Fatalf("bad profile accepted: %s", invalid)
		}
	}
}

func TestAllMerchantDiskProfilesValidatedWithoutOutbound(t *testing.T) {
	valid := map[string][]json.RawMessage{"system_disk_io_limit": {json.RawMessage(`"0,0,0,0"`), json.RawMessage(`"500,500,2000,2000"`)}}
	if err := ValidateOptionalProvisionChoices(json.RawMessage(`{}`), valid); err != nil {
		t.Fatal(err)
	}
	valid["system_disk_io_limit"] = append(valid["system_disk_io_limit"], json.RawMessage(`"bad"`))
	if err := ValidateOptionalProvisionChoices(json.RawMessage(`{}`), valid); err == nil {
		t.Fatal("invalid later choice accepted")
	}
	if err := ValidateOptionalProvisionChoices(json.RawMessage(`{}`), map[string][]json.RawMessage{"api_key": {json.RawMessage(`"secret"`)}}); err == nil {
		t.Fatal("secret alternative accepted")
	}
}

func TestTrafficCycleUsesNativeResetDay(t *testing.T) {
	for _, day := range []int{1, 21, 28, 29, 30, 31} {
		for cycle, want := range map[string]int64{"month": 1, "last_30days": int64(day)} {
			parameters := map[string]any{"traffic_bill_type": cycle}
			if err := appendTrafficResetDay(parameters, day); err != nil {
				t.Fatal(err)
			}
			if parameters["reset_flow_day"] != want {
				t.Fatalf("%s/%d mapped incorrectly", cycle, day)
			}
			if _, exists := parameters["traffic_bill_type"]; exists {
				t.Fatal("finance-only key leaked")
			}
		}
	}
	for _, day := range []int{0, 32} {
		if appendTrafficResetDay(map[string]any{"traffic_bill_type": "last_30days"}, day) == nil {
			t.Fatal("invalid native reset day accepted")
		}
	}
}
