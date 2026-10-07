package resourceinterfaceprojectioncap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectionRejectsSecretsAndNonPFINumbersByShape(t *testing.T) {
	t.Parallel()
	projection := Projection{
		InterfaceInstanceNo: "pfi_11111111111111111111111111111111", DisplayName: "魔方云 A",
		InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
		ActorUserID:                    101,
		OwnerID:                        OwnerID, ResourceChannelRef: "CHN_1", PluginID: "idc-zjmfcloud",
		PluginReleaseVersion: "1.0.0", ContractVersion: "v1", AuthorizedMethods: []string{"provision", "query"},
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), NewBusinessState: StateAccepting,
	}
	if err := ValidateProjection(projection); err != nil {
		t.Fatalf("ValidateProjection() error = %v", err)
	}
	projection.InterfaceInstanceNo = "CHN_1"
	if err := ValidateProjection(projection); err == nil {
		t.Fatal("ValidateProjection() accepted a resource channel as pfi identity")
	}
	projection.InterfaceInstanceNo = "pfi_11111111111111111111111111111111"
	projection.InterfaceConfigurationRevision = "rci_not-a-revision"
	if err := ValidateProjection(projection); err == nil {
		t.Fatal("ValidateProjection() accepted an invalid resource configuration revision")
	}
	projection.InterfaceConfigurationRevision = "rci_11111111111111111111111111111111"
	projection.ActorUserID = 0
	if err := ValidateProjection(projection); err == nil {
		t.Fatal("ValidateProjection() accepted a missing authenticated actor")
	}
	request := CandidateRequest{InterfaceInstanceNo: "pfi_11111111111111111111111111111111", Method: "candidate.region", Values: json.RawMessage(`{"country":"cn"}`)}
	if err := ValidateCandidateRequest(request); err != nil {
		t.Fatalf("ValidateCandidateRequest() error = %v", err)
	}
}
