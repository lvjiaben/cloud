package v10

import (
	"encoding/json"
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestBuildProvisionParametersFreezesProvenV10Facts(t *testing.T) {
	plan := resourcecap.ServerProvisionPlan{
		ExternalPrincipalRef: "42", Hostname: "vm-42",
		Selector:              json.RawMessage(`{"provision":{"provider":"idcsmart-business-v10","billing_cycle":"monthly","options":{"cpu":{"option_id":"1004","values":{"2":"2004"}}}}}`),
		ProductConfiguration:  json.RawMessage(`{"upstream_product_id":"81","upstream_region_option_id":"1001","upstream_line_option_id":"1002","upstream_os_option_id":"1003","region":"2001","line":"2002","cpu":2,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100}`),
		CustomerConfiguration: json.RawMessage(`{"operating_system":"2003"}`),
		Components:            []resourcecap.ServerProvisionComponent{{Type: resourcecap.ServerProvisionComponentCPU, Cores: 2, SelectionQuantity: 1}},
	}
	got, err := BuildProvisionParameters(plan)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(got, &value); err != nil {
		t.Fatal(err)
	}
	if value["pid"] != "81" || value["downstream_client_id"] != "42" || value["create_sub_account"] != float64(1) {
		t.Fatalf("unexpected parameters: %s", got)
	}
}

func TestCapabilitiesClaimOnlyReviewedV10Operations(t *testing.T) {
	snapshot := capabilities(time.Now().UTC())
	if err := snapshot.ValidateAt(snapshot.ObservedAt); err != nil {
		t.Fatalf("invalid capability snapshot: %v", err)
	}
	supportedCodes := map[resourcecap.CapabilityCode]bool{
		resourcecap.CapabilityAuthentication: true, resourcecap.CapabilityDiscovery: true,
		resourcecap.CapabilityResourceQuery: true, resourcecap.CapabilitySuspend: true,
		resourcecap.CapabilityResume: true, resourcecap.CapabilityTerminate: true,
	}
	for _, item := range snapshot.Capabilities {
		if supportedCodes[item.Code] && item.Support != resourcecap.SupportLevelSupported {
			t.Fatalf("reviewed V10 capability was not enabled: %+v", item)
		}
		if !supportedCodes[item.Code] && item.Support == resourcecap.SupportLevelSupported {
			t.Fatalf("unreviewed V10 capability was enabled: %+v", item)
		}
	}
}
