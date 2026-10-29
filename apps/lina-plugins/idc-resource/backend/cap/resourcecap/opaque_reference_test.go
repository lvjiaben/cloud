// This file checks generated provisioning identities against real validation.
package resourcecap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
)

func TestGeneratedServerReferenceValidation(t *testing.T) {
	for sample := range 512 {
		hash := sha256.Sum256([]byte(strconv.Itoa(sample)))
		value := hex.EncodeToString(hash[:])
		request := OperationRequest{RequestID: "core.provision.pft_" + value[:32], Capability: CapabilityProvision, ResourceRef: "SRV_" + value[:32], Parameters: json.RawMessage(`{"hostname":"srv-` + value[:16] + `.internal"}`)}
		if err := request.Validate(); err != nil {
			t.Fatalf("generated provisioning identity rejected at sample %d: %v", sample, err)
		}
		for _, prefix := range []string{"core.server-operation.nso_", "core.lifecycle.LCT_", "core.lifecycle.TLT_", "prt_"} {
			request.RequestID = prefix + value[:32]
			if err := request.Validate(); err != nil {
				t.Fatalf("generated operation identity %s rejected at sample %d: %v", prefix, sample, err)
			}
		}
		if err := ValidateOwnerMetadata(json.RawMessage(`{"operationNo":"pft_` + value[:32] + `","interfaceConfigurationRevision":"rci_` + value[:32] + `"}`)); err != nil {
			t.Fatalf("generated owner metadata rejected at sample %d: %v", sample, err)
		}
	}
	for _, value := range []string{"SRV_5feceb66ffc86f38d952786c6d696c79?api_key=hidden", "srv-5feceb66ffc86f38.internal/password=hidden", hex.EncodeToString([]byte("api_key=must-not-pass"))} {
		if err := ValidateOwnerMetadata(json.RawMessage(`{"note":` + strconv.Quote(value) + `}`)); err == nil {
			t.Fatal("credential-bearing text was accepted as an opaque identifier")
		}
	}
}
