package catalog

import (
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"testing"
)

func TestNativeBillingAccountPriceAndMarkup(t *testing.T) {
	term := channelcommercecap.BillingTerm{SubtotalAmount: "8199", SetupAmount: "500", TaxAmount: "0", TotalAmount: "8699"}
	amount, setup, err := nativeBillingAmounts(term, "10")
	if err != nil || amount != 9019 || setup != 550 {
		t.Fatalf("account amount %d setup %d err %v", amount, setup, err)
	}
	// Account totals are source evidence: no local fallback can repair bad tax,
	// missing fees, stale arithmetic or an overflowing source amount.
	for _, mutate := range []func(*channelcommercecap.BillingTerm){func(v *channelcommercecap.BillingTerm) { v.TotalAmount = "8700" }, func(v *channelcommercecap.BillingTerm) { v.TaxAmount = "1" }, func(v *channelcommercecap.BillingTerm) { v.SetupAmount = "" }, func(v *channelcommercecap.BillingTerm) { v.SubtotalAmount = "9223372036854775808" }} {
		changed := term
		mutate(&changed)
		if _, _, err = nativeBillingAmounts(changed, "10"); err == nil {
			t.Fatal("accepted non-authoritative account price")
		}
	}
}
func TestNativeBillingScheduleIdentity(t *testing.T) {
	snapshot := channelcommercecap.ImportSnapshot{ConfigurationRevision: "source-revision"}
	term := channelcommercecap.BillingTerm{BillingTermToken: "cycle-12", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 12, MinimumPurchaseCycles: 1, Status: "available", SetupAmount: "0", SubtotalAmount: "100", TaxAmount: "0", TotalAmount: "100"}
	live := &channelcommercecap.BillingTerms{ConfigurationRevision: snapshot.ConfigurationRevision, CurrencyCode: "USD", Terms: []channelcommercecap.BillingTerm{term}}
	definition := BillingRuleDefinition{ChargeType: ChargeTypeRecurring, PeriodUnit: PeriodUnitMonth, PeriodCount: 12}
	if _, err := nativeBillingTerm(snapshot, live, "USD", "cycle-12", definition); err != nil {
		t.Fatal(err)
	}
	live.Terms[0].PeriodCount = 1
	if _, err := nativeBillingTerm(snapshot, live, "USD", "cycle-12", definition); err == nil {
		t.Fatal("accepted changed source period")
	}
}

func TestImportedOneTimePolicyAndDisabledFixedPrice(t *testing.T) {
	if importedPolicyPeriod(string(ChargeTypeOneTime), "") != "once" {
		t.Fatal("one-time saved price lost its billing identity")
	}
	setup := int64(9)
	if _, err := nativePolicySetup(nativeMarkup{fixedSetup: true, savedSetup: &setup, status: "inactive"}, 50); err == nil {
		t.Fatal("inactive native fixed price became sellable")
	}
	if amount, err := nativePolicySetup(nativeMarkup{fixedSetup: true, savedSetup: &setup, status: "active"}, 50); err != nil || amount != 9 {
		t.Fatal("merchant setup override ignored")
	}
	if amount, err := nativePolicySetup(nativeMarkup{}, 50); err != nil || amount != 50 {
		t.Fatal("remote term without static price was disabled")
	}
}
