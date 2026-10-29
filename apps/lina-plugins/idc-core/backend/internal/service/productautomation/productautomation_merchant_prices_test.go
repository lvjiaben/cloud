// These tests prove generic merchant price identity, quantity semantics and
// rejection boundaries without a supplier, credentials or a database.
package productautomation

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"testing"
)

func TestMerchantPriceTargetsBindExactChoiceAndQuantity(t *testing.T) {
	candidates := []configurationPriceCandidatePrivate{
		{ConfigurationPriceCandidate: ConfigurationPriceCandidate{CandidateToken: "opaque-validation"}, fieldKey: "validation_method", candidateValue: "dns"},
		{ConfigurationPriceCandidate: ConfigurationPriceCandidate{CandidateToken: "opaque-storage"}, fieldKey: "storage_profile", candidateValue: "500,500,2000,2000"},
		{ConfigurationPriceCandidate: ConfigurationPriceCandidate{CandidateToken: "opaque-seats", Quantity: true}, fieldKey: "seats", candidateValue: configurationQuantityPriceIdentity},
	}
	selected, modes, aliases, err := selectMerchantPriceTargets(candidates, []MerchantPriceTargetInput{
		{Token: "monthly", FieldKey: "validation_method", CandidateValue: "dns"},
		{Token: "yearly", FieldKey: "validation_method", CandidateValue: "dns"},
		{Token: "profile", FieldKey: "storage_profile", CandidateValue: "500,500,2000,2000"},
		{Token: "seats", FieldKey: "seats", Quantity: true, PricingMode: catalog.ConfigurationPriceBandTotal},
	})
	if err != nil || len(selected) != 3 || aliases["yearly"] != "monthly" || modes["monthly"] != catalog.ConfigurationPriceFixed || modes["seats"] != catalog.ConfigurationPriceBandTotal {
		t.Fatalf("generic targets lost identity or mode: selected=%+v modes=%+v aliases=%+v err=%v", selected, modes, aliases, err)
	}
	if selected[1].candidateValue != "500,500,2000,2000" || selected[2].candidateValue != configurationQuantityPriceIdentity {
		t.Fatal("choice or quantity value changed")
	}
	if candidates[0].CandidateToken != "opaque-validation" {
		t.Fatal("saved candidates mutated")
	}
}

func TestMerchantPriceTargetsRejectInventedTargetsAndConflictingModes(t *testing.T) {
	candidates := []configurationPriceCandidatePrivate{
		{fieldKey: "validation_method", candidateValue: "dns"},
		{ConfigurationPriceCandidate: ConfigurationPriceCandidate{Quantity: true}, fieldKey: "seats", candidateValue: configurationQuantityPriceIdentity},
	}
	cases := [][]MerchantPriceTargetInput{
		{},
		{{Token: "x", FieldKey: "password", CandidateValue: "secret"}},
		{{Token: "x", FieldKey: "validation_method", CandidateValue: "file"}},
		{{Token: "x", FieldKey: "validation_method", CandidateValue: "dns", PricingMode: catalog.ConfigurationPricePerUnit}},
		{{Token: "x", FieldKey: "seats", Quantity: true, PricingMode: catalog.ConfigurationPriceFixed}},
		{{Token: "x", FieldKey: "seats", Quantity: true, CandidateValue: "1"}},
		{{Token: "x", FieldKey: "seats", CandidateValue: configurationQuantityPriceIdentity}},
		{{FieldKey: "validation_method", CandidateValue: "dns"}},
		{{Token: "same", FieldKey: "validation_method", CandidateValue: "dns"}, {Token: "same", FieldKey: "seats", Quantity: true}},
		{{Token: "a", FieldKey: "seats", Quantity: true, PricingMode: catalog.ConfigurationPricePerUnit}, {Token: "b", FieldKey: "seats", Quantity: true, PricingMode: catalog.ConfigurationPriceBandTotal}},
	}
	for index, inputs := range cases {
		if _, _, _, err := selectMerchantPriceTargets(candidates, inputs); err == nil {
			t.Fatalf("invalid case %d accepted", index)
		}
	}
	inputs := make([]MerchantPriceTargetInput, 257)
	if _, _, _, err := selectMerchantPriceTargets(candidates, inputs); err == nil {
		t.Fatal("unbounded price rows accepted")
	}
}

func TestMerchantPriceProjectionOnlyIncludesCurrentSafeFields(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "配置", EnUS: "Configuration"}
	configuration := &VersionConfiguration{Version: 3, CustomerDeclaration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "validation_method", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "dns", Label: label}}},
		{Key: "seats", Type: capmodel.ConfigurationFieldInteger, Label: label, Minimum: "1", Maximum: "10"},
		{Key: "password", Type: capmodel.ConfigurationFieldSecret, Label: label},
	}}}
	rows := []configurationPriceDimensionRow{
		{DimensionNo: "choice", FieldKey: "validation_method", CandidateValue: "dns", PricingMode: catalog.ConfigurationPriceFixed},
		{DimensionNo: "old-choice", FieldKey: "validation_method", CandidateValue: "deleted", PricingMode: catalog.ConfigurationPriceFixed},
		{DimensionNo: "quantity", FieldKey: "seats", CandidateValue: configurationQuantityPriceIdentity, PricingMode: catalog.ConfigurationPricePerUnit},
		{DimensionNo: "secret", FieldKey: "password", CandidateValue: "secret", PricingMode: catalog.ConfigurationPriceFixed},
		{DimensionNo: "gone", FieldKey: "removed", CandidateValue: "old", PricingMode: catalog.ConfigurationPriceFixed},
	}
	projected := projectMerchantPriceTargets(configuration, rows)
	if len(projected) != 2 || projected[0].FieldKey != "validation_method" || projected[0].CandidateValue != "dns" || !projected[1].Quantity || projected[1].CandidateValue != "" {
		t.Fatalf("unsafe or missing projection: %+v", projected)
	}
	configuration.CustomerFieldSelections = []CustomerFieldSelection{{FieldKey: "seats", NumericPricing: configurationcontract.NumericPricingChoice, NumericChoices: []string{"2", "4"}}}
	if result := projectMerchantPriceTargets(configuration, rows); len(result) != 1 {
		t.Fatalf("old quantity target survived per-choice configuration: %+v", result)
	}
}
