// Atomic offer pricing keeps fixed/base prices distinct from quantity bands.
package catalog

import (
	"lina-plugin-idc-core/backend/internal/model/entity"
	"testing"
)

func TestConfigurationPriceBatchValidatesDeclaredQuantityMode(t *testing.T) {
	const version = "ver_11111111111111111111111111111111"
	rule := &entity.CatalogBillingRule{BillingRuleNo: "monthly", VersionNo: version, ChargeTypeCode: string(ChargeTypeRecurring), PeriodUnitCode: string(PeriodUnitMonth), PeriodCount: 1, PricePhaseCode: string(PricePhaseBoth), QuantityModeCode: string(QuantityModeFixed), MinimumPurchaseCycles: 1, StatusCode: "active"}
	maximum := int64(10)
	zero, hundred := int64(0), int64(100)
	row := ConfigurationPriceBatchRow{PricingMode: ConfigurationPricePerUnit, CandidateToken: "quantity", Schedule: PriceScheduleCreateInput{BillingRuleNo: "monthly", CurrencyCode: "CNY", Active: true, Tiers: []PriceTierDefinition{
		{MinimumQuantity: 0, MaximumQuantity: &maximum, AmountMinor: 0, RenewalAmountMinor: &zero},
		{MinimumQuantity: 10, AmountMinor: 100, RenewalAmountMinor: &hundred},
	}}}
	rules := map[string]*entity.CatalogBillingRule{"monthly": rule}
	for _, mode := range []ConfigurationPriceMode{ConfigurationPricePerUnit, ConfigurationPriceBandTotal} {
		row.PricingMode = mode
		if _, err := normalizeConfigurationPriceBatch(version, []ConfigurationPriceBatchRow{row}, map[string]string{"quantity": "cpd_quantity"}, rules, nil); err != nil {
			t.Fatalf("quantity schedule rejected: %s %v", mode, err)
		}
	}
	row.PricingMode = ConfigurationPriceFixed
	if _, err := normalizeConfigurationPriceBatch(version, []ConfigurationPriceBatchRow{row}, map[string]string{"quantity": "cpd_quantity"}, rules, nil); err == nil {
		t.Fatal("quantity bands accepted as a fixed price")
	}
	row.PricingMode = ConfigurationPricePerUnit
	row.TargetKind = PriceBatchTargetBase
	row.CandidateToken = ""
	if _, err := normalizeConfigurationPriceBatch(version, []ConfigurationPriceBatchRow{row}, nil, rules, nil); err == nil {
		t.Fatal("configuration quantity algorithm applied to base price")
	}
}
