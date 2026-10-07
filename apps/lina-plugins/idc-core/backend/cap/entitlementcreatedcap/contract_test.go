package entitlementcreatedcap

import "testing"

func TestContractRejectsMismatchedOpaqueReceipt(t *testing.T) {
	event := Event{Version: ContractVersion, EventNo: "evt-1", EntitlementNo: "ent-1", IntentNo: "int-1",
		CustomerNo: "cus-1", ProductNo: "prd-1", ProductVersionNo: "ver-1", ConfigurationSnapshotNo: "cfg-1",
		ProductNameZhCN: "证书", ProductNameEnUS: "Certificate",
		InterfaceInstanceNo: "ifc-1", PluginID: "ssl-provider", PluginReleaseVersion: "1.0.0",
		ContractVersion: "v1", DeclarationDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", OwnerID: "ssl-owner"}
	if !ValidateEvent(event) {
		t.Fatal("valid entitlement event rejected")
	}
	receipt := Receipt{Version: ContractVersion, EventNo: "other", OwnerReference: "opaque:123",
		EvidenceDigest: event.DeclarationDigest}
	if ValidateReceipt(event, receipt) {
		t.Fatal("mismatched receipt accepted")
	}
}

func TestContractRejectsMissingFrozenOwner(t *testing.T) {
	event := Event{Version: ContractVersion, EventNo: "evt-1", EntitlementNo: "ent-1", IntentNo: "int-1",
		CustomerNo: "cus-1", ProductNo: "prd-1", ProductVersionNo: "ver-1", ConfigurationSnapshotNo: "cfg-1",
		ProductNameZhCN: "服务器", ProductNameEnUS: "Server",
		InterfaceInstanceNo: "ifc-1", PluginID: "server-connector", PluginReleaseVersion: "1.0.0",
		ContractVersion: "v1", DeclarationDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	if ValidateEvent(event) {
		t.Fatal("event without the published business owner was accepted")
	}
}
