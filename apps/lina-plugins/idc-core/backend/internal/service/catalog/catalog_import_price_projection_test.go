package catalog

import (
	"bytes"
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestProjectImportPricesKeepsBaseAndOpaqueLegacyOption(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	rate := 600
	renewal := int64(90)
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "region", Type: capmodel.ConfigurationFieldSingleSelect,
		Label:            &capmodel.LocalizedText{EnUS: "Region", ZhCN: "区域"},
		StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "us", Label: &capmodel.LocalizedText{EnUS: "US", ZhCN: "美国"}}},
	}}}
	snapshot := &publicStructuredConfigurationSnapshot{Target: publicStructuredConfigurationTarget{VersionNo: "v1"}, Revision: "revision-a", Schema: schema}
	projection := &importSchemaProjection{schema: schema, fields: map[string]string{"region": "field_opaque"}, codec: codec, revision: snapshot.Revision}
	source := &publicQuoteSource{
		BillingRules:  []entity.CatalogBillingRule{{BillingRuleNo: "rule", VersionNo: "v1", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, PricePhaseCode: "both", QuantityModeCode: "fixed", MinimumPurchaseCycles: 1}},
		Prices:        []entity.CatalogPrice{{VersionNo: "v1", BillingRuleNo: "rule", CurrencyCode: "USD", AmountMinor: 100, RenewalAmountMinor: &renewal}, {VersionNo: "v1", BillingRuleNo: "rule", OptionNo: stringPtr("legacy"), CurrencyCode: "USD", AmountMinor: 120, RenewalAmountMinor: &renewal}},
		TaxPolicies:   []entity.CatalogTaxPolicy{{CurrencyCode: "USD", PriceIncludesTax: true, TaxTreatmentCode: "taxable", TaxRateBasisPoints: &rate, TaxCategoryCode: "cloud_service", RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1}},
		LegacyOptions: []importLegacyOptionMapping{{OptionNo: "legacy", FieldKey: "region", NormalizedValue: `{"value":"us"}`}},
	}
	result, err := projectImportPrices(codec, "demo", snapshot, projection, source, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || len(result[0].Base) != 1 || len(result[0].Options) != 1 || result[0].Options[0].FieldKey != "field_opaque" || result[0].Options[0].Value == "us" {
		t.Fatalf("unexpected projection: %#v", result)
	}
}

func TestProjectImportPricesRejectsLegacyOptionWithoutValue(t *testing.T) {
	codec, _ := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{4}, 32))
	snapshot := &publicStructuredConfigurationSnapshot{Target: publicStructuredConfigurationTarget{VersionNo: "v1"}, Revision: "revision-a", Schema: &capmodel.ConfigurationSchemaDeclaration{}}
	projection := &importSchemaProjection{fields: map[string]string{}, codec: codec, revision: snapshot.Revision}
	source := &publicQuoteSource{BillingRules: []entity.CatalogBillingRule{{BillingRuleNo: "rule", VersionNo: "v1", ChargeTypeCode: "one_time", PeriodUnitCode: "once", PeriodCount: 1, PricePhaseCode: "new_purchase", QuantityModeCode: "fixed", MinimumPurchaseCycles: 1}}, Prices: []entity.CatalogPrice{{VersionNo: "v1", BillingRuleNo: "rule", OptionNo: stringPtr("legacy"), CurrencyCode: "USD", AmountMinor: 1}}, LegacyOptions: []importLegacyOptionMapping{{OptionNo: "legacy", FieldKey: "region", NormalizedValue: `{"cores":2}`}}}
	if _, err := projectImportPrices(codec, "demo", snapshot, projection, source, nil, nil); err == nil {
		t.Fatal("expected missing legacy value to fail closed")
	}
}

func stringPtr(value string) *string { return &value }

// TestProjectImportQuantityPrices freezes the algorithm and schema limits
// across portable export, JSON transport and downstream price persistence.
func TestProjectImportQuantityPrices(t *testing.T) {
	for _, mode := range []ConfigurationPriceMode{ConfigurationPricePerUnit, ConfigurationPriceBandTotal} {
		t.Run(string(mode), func(t *testing.T) {
			codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{3}, 32))
			if err != nil {
				t.Fatal(err)
			}
			label := &capmodel.LocalizedText{ZhCN: "数据盘", EnUS: "Data disk"}
			schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "disk", Type: capmodel.ConfigurationFieldInteger, Label: label}}}
			snapshot := &publicStructuredConfigurationSnapshot{Target: publicStructuredConfigurationTarget{VersionNo: "v1"}, Revision: "revision-a", Schema: schema}
			projection := &importSchemaProjection{schema: schema, fields: map[string]string{"disk": "field_disk"}, codec: codec, revision: snapshot.Revision}
			maximum, renewal := int64(100), int64(1)
			source := &publicQuoteSource{
				BillingRules: []entity.CatalogBillingRule{{BillingRuleNo: "rule", VersionNo: "v1", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, PricePhaseCode: "both", QuantityModeCode: "fixed", MinimumPurchaseCycles: 1}},
				Prices: []entity.CatalogPrice{{VersionNo: "v1", BillingRuleNo: "rule", CurrencyCode: "USD", AmountMinor: 100, RenewalAmountMinor: &renewal},
					{VersionNo: "v1", BillingRuleNo: "rule", CurrencyCode: "USD", ConfigurationPriceDimensionNo: "cpd_disk", MaximumQuantity: &maximum, AmountMinor: 2, SetupFeeMinor: 3, RenewalAmountMinor: &renewal},
					{VersionNo: "v1", BillingRuleNo: "rule", CurrencyCode: "USD", ConfigurationPriceDimensionNo: "cpd_disk", MinimumQuantity: 100, AmountMinor: 1, SetupFeeMinor: 5, RenewalAmountMinor: &renewal}},
				TaxPolicies: []entity.CatalogTaxPolicy{{CurrencyCode: "USD", TaxTreatmentCode: "exempt", TaxCategoryCode: "service", RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1}},
			}
			result, err := projectImportPrices(codec, "demo", snapshot, projection, source, []configurationPriceDimensionIdentity{{DimensionNo: "cpd_disk", FieldKey: "disk", CandidateValue: "quantity", PricingMode: mode}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var transported []channelcommercecap.ImportPriceSchedule
			if err = json.Unmarshal(encoded, &transported); err != nil {
				t.Fatal(err)
			}
			option := transported[0].Options[0]
			if ConfigurationPriceMode(option.PricingMode) != mode || option.Value != "" || len(option.Bands) != 2 || option.Bands[1].SetupAmountMinor != 5 {
				t.Fatalf("quantity algorithm lost: %+v", option)
			}
			downstream := &channelcommercecap.ImportSnapshot{Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "field_disk", Type: capmodel.ConfigurationFieldInteger, Label: label}}}}
			svc := &managementServiceImpl{random: bytes.NewReader(bytes.Repeat([]byte{1}, 1024))}
			rows, err := svc.portableImportRows("downstream-version", "0", downstream, []importPrice{{portable: &transported[0]}})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows.prices) != 3 || len(rows.dimensionSQL) != 1 || rows.dimensionArgs[len(rows.dimensionArgs)-1] != mode {
				t.Fatalf("quantity persistence lost: %+v", rows)
			}
			conflict := transported[0]
			conflict.TermReference = "different-term"
			conflict.Options = append([]channelcommercecap.ImportOptionPrice(nil), conflict.Options...)
			conflict.Options[0].PricingMode = channelcommercecap.ImportConfigurationPriceFixed
			conflict.Options[0].Value = "quantity"
			if _, err = svc.portableImportRows("downstream-version", "0", downstream, []importPrice{{portable: &transported[0]}, {portable: &conflict}}); err == nil {
				t.Fatal("conflicting algorithms accepted")
			}
		})
	}
}
