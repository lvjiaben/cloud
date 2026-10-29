// This file verifies canonical option keys exclude irrelevant customer inputs.
package interfacecandidate

import (
	"bytes"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/entity"
	"strings"
	"testing"
)

func TestCandidateSnapshotRequestCanonicalizesOnlyConsumedRouting(t *testing.T) {
	first, _, err := candidateSnapshotRequest([]byte(`{"schemaVersion":"v1","currentSelections":[{"cpu":2}],"requests":[{"fieldKey":"line","parentValues":["1"],"selectedValues":["22"]},{"fieldKey":"region"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := candidateSnapshotRequest([]byte(`{"schemaVersion":"v1","currentSelections":[{"cpu":16}],"requests":[{"fieldKey":"region"},{"fieldKey":"line","parentValues":["1"]}]}`))
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("unconsumed selection created another supplier query: %s / %s", first, second)
	}
	third, _, err := candidateSnapshotRequest([]byte(`{"schemaVersion":"v1","requests":[{"fieldKey":"region"},{"fieldKey":"line","parentValues":["2"]}]}`))
	if err != nil || bytes.Equal(first, third) {
		t.Fatal("different regions shared a routing projection")
	}
	for _, invalid := range []string{
		`{"schemaVersion":"v1","secret":"not-stored","requests":[{"fieldKey":"region"}]}`,
		`{"schemaVersion":"v1","requests":[{"fieldKey":"region","parentValues":["1"]}]}`,
		`{"schemaVersion":"v1","requests":[{"fieldKey":"line","parentValues":["1","2"]}]}`,
		`{"schemaVersion":"v1","requests":[{"fieldKey":"region"},{"fieldKey":"region"}]}`,
		`{"schemaVersion":"v1","requests":[{"fieldKey":"password"}]}`,
	} {
		if _, _, err = candidateSnapshotRequest([]byte(invalid)); err == nil {
			t.Fatal("invalid supplier option request was retained")
		}
	}
}

func TestRetainedDirectoryRequiresUnchangedRevisionAndCurrentAuthority(t *testing.T) {
	current := binding{PluginID: "idc-zjmfcloud", ContractVersion: resourcecap.ServerFulfillmentContractVersion, ConfigurationRevision: "revision-a"}
	evidence := &capmodel.BusinessExtensionProviderEvidence{PluginID: current.PluginID, Point: resourcecap.DynamicProviderPoint, ContractVersion: current.ContractVersion, ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{resourcecap.DynamicProviderMethodConfigurationCandidates}}
	previous := &entity.InterfaceCandidateSnapshot{ConfigurationRevision: "revision-a"}
	if !canRetainCandidateDirectory(current, evidence, previous) {
		t.Fatal("same authorized connection could not retain its directory")
	}
	for _, change := range []func(*binding, *capmodel.BusinessExtensionProviderEvidence, *entity.InterfaceCandidateSnapshot){
		func(b *binding, _ *capmodel.BusinessExtensionProviderEvidence, _ *entity.InterfaceCandidateSnapshot) {
			b.ConfigurationRevision = "different-account"
		},
		func(b *binding, _ *capmodel.BusinessExtensionProviderEvidence, _ *entity.InterfaceCandidateSnapshot) {
			b.ContractVersion = "v2"
		},
		func(_ *binding, e *capmodel.BusinessExtensionProviderEvidence, _ *entity.InterfaceCandidateSnapshot) {
			e.AuthorizedMethods = nil
		},
		func(_ *binding, e *capmodel.BusinessExtensionProviderEvidence, _ *entity.InterfaceCandidateSnapshot) {
			e.ReleaseDigest = "invalid"
		},
	} {
		b, e, p := current, *evidence, *previous
		change(&b, &e, &p)
		if canRetainCandidateDirectory(b, &e, &p) {
			t.Fatal("directory was reused across a changed boundary")
		}
	}
}
