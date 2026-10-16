// These tests prevent procurement from accepting incomplete or contradictory
// frozen terms, including accidental month/year and recurring/one-time mixing.

package productfulfillmentcap

import "testing"

func TestBillingScheduleValidation(t *testing.T) {
	valid := BillingSchedule{CurrencyCode: "CNY", ChargeType: BillingChargeRecurring, PeriodUnit: BillingPeriodMonth, PeriodCount: 3, PurchaseCycles: 2}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*BillingSchedule){
		"missing currency":      func(b *BillingSchedule) { b.CurrencyCode = "" },
		"unknown period":        func(b *BillingSchedule) { b.PeriodUnit = "translated-label" },
		"missing interval":      func(b *BillingSchedule) { b.PeriodCount = 0 },
		"unbounded interval":    func(b *BillingSchedule) { b.PeriodUnit = BillingPeriodYear; b.PeriodCount = 11 },
		"missing cycles":        func(b *BillingSchedule) { b.PurchaseCycles = 0 },
		"recurring once":        func(b *BillingSchedule) { b.PeriodUnit = BillingPeriodOnce; b.PeriodCount = 1 },
		"one-time month":        func(b *BillingSchedule) { b.ChargeType = BillingChargeOneTime },
		"multiple trial cycles": func(b *BillingSchedule) { b.ChargeType = BillingChargeTrial },
	} {
		t.Run(name, func(t *testing.T) {
			changed := valid
			change(&changed)
			if changed.Validate() == nil {
				t.Fatal("invalid procurement terms accepted")
			}
		})
	}
}
