package trialqualification

import (
	"slices"
	"testing"
)

func TestEvidenceDigestsCanonicalizeSetsWithoutMutatingInput(t *testing.T) {
	movement := MovementEvidenceFact{
		MovementType: "hold", OrderNo: "order-1", ScopeNo: "scope-1",
		Entitlements: []string{"digest-b", "digest-a"},
	}
	originalEntitlements := slices.Clone(movement.Entitlements)
	first, err := MovementEvidenceDigest(movement)
	if err != nil {
		t.Fatalf("digest movement evidence: %v", err)
	}
	if !slices.Equal(movement.Entitlements, originalEntitlements) {
		t.Fatalf("movement evidence input was mutated: %v", movement.Entitlements)
	}
	movement.Entitlements = []string{"digest-a", "digest-b"}
	second, err := MovementEvidenceDigest(movement)
	if err != nil || first != second {
		t.Fatalf("movement evidence is order-dependent: first=%q second=%q err=%v", first, second, err)
	}

	settlement := SettlementEvidenceFact{
		OrderNo: "order-1", InvoiceNo: "invoice-1", CustomerNo: "customer-1",
		IdentityEvidenceDigest: "identity-digest",
		Entitlements:           []string{"entitlement-b", "entitlement-a"},
		HoldMovements:          []string{"hold-b", "hold-a"},
		ConsumeMovements:       []string{"consume-b", "consume-a"},
	}
	originalSettlementEntitlements := slices.Clone(settlement.Entitlements)
	first, err = SettlementEvidenceDigest(settlement)
	if err != nil {
		t.Fatalf("digest settlement evidence: %v", err)
	}
	if !slices.Equal(settlement.Entitlements, originalSettlementEntitlements) {
		t.Fatalf("settlement evidence input was mutated: %v", settlement.Entitlements)
	}
	settlement.Entitlements = []string{"entitlement-a", "entitlement-b"}
	settlement.HoldMovements = []string{"hold-a", "hold-b"}
	settlement.ConsumeMovements = []string{"consume-a", "consume-b"}
	second, err = SettlementEvidenceDigest(settlement)
	if err != nil || first != second {
		t.Fatalf("settlement evidence is order-dependent: first=%q second=%q err=%v", first, second, err)
	}
}
