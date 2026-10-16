// This file verifies that existing-business transport cannot borrow a different
// interface, provider release, contract or method's credential authority.

package interfacecandidate

import (
	"testing"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func TestExistingBusinessRequiresEveryFrozenProviderIdentity(t *testing.T) {
	value := binding{ChannelNo: "CHN_peer", ProductPluginID: "idc-peer", ProductReleaseVersion: "v1", ProductContractVersion: "v1", ProductReleaseDigest: "release", ProductDeclarationDigest: "declaration", ProductAuthorizedMethods: []string{"provision", "query"}}
	valid := resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest{ResourceChannelRef: "CHN_peer", PluginID: "idc-peer", PluginReleaseVersion: "v1", ContractVersion: "v1", ReleaseDigest: "release", DeclarationDigest: "declaration", Method: "query"}
	if !matchesExistingBusiness(value, valid) {
		t.Fatal("matching frozen evidence was rejected")
	}
	cases := []struct {
		name   string
		mutate func(*resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest)
	}{
		{"channel", func(v *resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) {
			v.ResourceChannelRef = "CHN_other"
		}},
		{"plugin", func(v *resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) { v.PluginID = "other" }},
		{"release", func(v *resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) { v.PluginReleaseVersion = "v2" }},
		{"contract", func(v *resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) { v.ContractVersion = "v2" }},
		{"release digest", func(v *resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) { v.ReleaseDigest = "other" }},
		{"declaration digest", func(v *resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) { v.DeclarationDigest = "other" }},
		{"method", func(v *resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) { v.Method = "renew" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := valid
			test.mutate(&request)
			if matchesExistingBusiness(value, request) {
				t.Fatal("mismatched authority accepted")
			}
		})
	}
}
