package finance

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestNativeFinanceCapabilityMatrixIsComplete(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	if err := DeclaredCapabilitySnapshot(now).ValidateAt(now); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionParametersFreezeUpstreamProductAndOptionMapping(t *testing.T) {
	plan := resourcecap.ServerProvisionPlan{
		ExternalPrincipalRef: "customer-1", Hostname: "server-1",
		Selector:              json.RawMessage(`{"provision":{"provider":"zjmf-finance-native","billing_cycle":"monthly","options":{"cpu":{"option_id":"11","values":{"2":"102"}}}}}`),
		ProductConfiguration:  json.RawMessage(`{"upstream_product_id":"42","upstream_region_option_id":"21","upstream_line_option_id":"22","upstream_os_option_id":"23","region":"301","line":"302","cpu":2,"memory":2048,"system_disk":40,"data_disk":0,"bandwidth":10}`),
		CustomerConfiguration: json.RawMessage(`{"operating_system":"204"}`),
		Components: []resourcecap.ServerProvisionComponent{
			{Type: resourcecap.ServerProvisionComponentCPU, Cores: 2, SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentOperatingSystem, Code: "204", SelectionQuantity: 1},
		},
	}
	result, err := BuildProvisionParameters(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if json.Unmarshal(result, &value) != nil {
		t.Fatalf("invalid frozen parameters: %s", result)
	}
	options, _ := value["configoption"].(map[string]any)
	if value["pid"] != "42" || value["billingcycle"] != "monthly" || options["23"] != "204" {
		t.Fatalf("unexpected frozen parameters: %s", result)
	}
}

func TestLifecycleUsesOnlyVerifiedNativeRoutes(t *testing.T) {
	request := resourcecap.DynamicOperationRequest{RequestID: "request-1", Capability: resourcecap.CapabilityReboot, ResourceRef: "91", Parameters: json.RawMessage(`{"mode":"hard"}`)}
	call, err := LifecycleCall(request)
	if err != nil || call.Path != "/provision/default" || call.Form.Get("func") != "hard_reboot" || call.Form.Get("is_api") != "1" {
		t.Fatalf("unexpected lifecycle call: %#v err=%v", call, err)
	}
	request.Capability = resourcecap.CapabilityResize
	if _, err = LifecycleCall(request); err == nil {
		t.Fatal("unverified billing operation was accepted")
	}
}

func TestUnknownPowerStateFailsClosed(t *testing.T) {
	if got := NormalizePowerState("vendor-new-state"); got != resourcecap.ResourceStateUnknown {
		t.Fatalf("unknown state became %q", got)
	}
}
