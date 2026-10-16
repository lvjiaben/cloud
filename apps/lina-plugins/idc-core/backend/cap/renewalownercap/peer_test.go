// This file verifies the peer renewal envelope cannot change owner or submit an
// unbounded/non-recurring term before any provider invocation is attempted.
package renewalownercap

import (
	"strings"
	"testing"
	"time"
)

func TestPeerRequestValidatesFrozenContract(t *testing.T) {
	original := PeerRequest{Request: TaskRequest{Version: ContractVersion, OwnerID: PeerOwnerID, OperationNo: "op", EntitlementNo: "ent", PriceSnapshotNo: "price", PluginID: PeerOwnerID, PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", DeclarationDigest: strings.Repeat("a", 64), RenewalMethod: PeerRenewMethod, ExpectedEntitlementVersion: 1, OriginalExpiresAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ProjectedExpiresAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}, ExternalReference: "peer-order:42", UpstreamServiceNo: "service-second", PurchaseCycles: 1, CurrencyCode: "CNY", PeriodUnitCode: "month", PeriodCount: 1}
	if err := ValidatePeerRequest(original); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*PeerRequest){"owner": func(r *PeerRequest) { r.Request.OwnerID = "other" }, "method": func(r *PeerRequest) { r.Request.RenewalMethod = "provision" }, "missing-service": func(r *PeerRequest) { r.UpstreamServiceNo = "" }, "missing-reference": func(r *PeerRequest) { r.ExternalReference = "" }, "cycles": func(r *PeerRequest) { r.PurchaseCycles = 121 }, "once": func(r *PeerRequest) { r.PeriodUnitCode = "once" }, "currency": func(r *PeerRequest) { r.CurrencyCode = "" }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			input := original
			mutate(&input)
			if err := ValidatePeerRequest(input); err == nil {
				t.Fatal("invalid renewal envelope accepted")
			}
		})
	}
}
