package resourcecap

import (
	"strings"
	"testing"
)

func TestServerEntitlementBindingRequiresFrozenOwnerAndServerEvidence(t *testing.T) {
	binding := validServerEntitlementBinding()
	if err := binding.Validate(); err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*ServerEntitlementBinding)
	}{
		{name: "customer", mutate: func(value *ServerEntitlementBinding) { value.CustomerNo = "" }},
		{name: "resource identity", mutate: func(value *ServerEntitlementBinding) { value.LocalResourceNo = "" }},
		{name: "server contract", mutate: func(value *ServerEntitlementBinding) { value.ServerContractVersion = "phone-v1" }},
		{name: "plugin", mutate: func(value *ServerEntitlementBinding) { value.ProviderPluginID = "Phone Supplier" }},
		{name: "release digest", mutate: func(value *ServerEntitlementBinding) { value.ReleaseDigest = "not-a-digest" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := binding
			test.mutate(&changed)
			if err := changed.Validate(); err == nil {
				t.Fatal("invalid server ownership binding was accepted")
			}
		})
	}
}

func validServerEntitlementBinding() ServerEntitlementBinding {
	return ServerEntitlementBinding{
		Version:                    ServerEntitlementProjectionVersion,
		EntitlementNo:              "ent_0123456789abcdef0123456789abcdef",
		SourceIntentNo:             "pfi_0123456789abcdef0123456789abcdef",
		CustomerNo:                 "10000",
		ProductNo:                  "product_server_1",
		ProductVersionNo:           "version_server_1",
		ProductNameZhCN:            "云服务器 1",
		ProductNameEnUS:            "Cloud server 1",
		LocalResourceNo:            "SRV_0123456789abcdef0123456789abcdef",
		MappingNo:                  "mapping_0123456789abcdef0123456789abcdef",
		ProviderTaskNo:             "task_0123456789abcdef0123456789abcdef",
		InterfaceInstanceNo:        "pfi_abcdef0123456789abcdef0123456789",
		ProviderPluginID:           "idc-zjmfcloud",
		PluginReleaseVersion:       "v1.0.0",
		FulfillmentContractVersion: "v1",
		ServerContractVersion:      ServerFulfillmentContractVersion,
		ReleaseDigest:              strings.Repeat("a", 64),
		DeclarationDigest:          strings.Repeat("b", 64),
	}
}
