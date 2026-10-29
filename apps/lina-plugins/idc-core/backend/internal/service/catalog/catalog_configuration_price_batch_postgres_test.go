package catalog

import (
	"context"
	"crypto/rand"
	"os"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

type fixedPriceTestTargets map[string]string

func (f fixedPriceTestTargets) ResolvePriceTargets(_ context.Context, _ string, _ []string) (map[string]string, error) {
	return f, nil
}

func TestConfigurationPriceTableAtomicPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	for _, table := range []string{"catalog_product_version", "catalog_billing_rule", "catalog_price", "catalog_configuration_price_dimension", "catalog_import_contract"} {
		if _, err := db.Exec(ctx, "CREATE TABLE plugin_idc_core_"+table+" (LIKE public.plugin_idc_core_"+table+" INCLUDING DEFAULTS INCLUDING IDENTITY)"); err != nil {
			t.Fatal(err)
		}
	}
	const version = "ver_11111111111111111111111111111111"
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code,source_version,source_fingerprint,created_by) VALUES(?,'prd_fixture',1,'draft','manual','1',repeat('a',64),81)`, version); err != nil {
		t.Fatal(err)
	}
	audit := &scheduleMutationAudit{}
	svc, err := newManagementService(audit, &managementTestAuthorizer{actor: 81}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := svc.CreateBillingRule(ctx, BillingRuleCreateInput{VersionNo: version, RuleCode: "once", Active: true, Definition: BillingRuleDefinition{ChargeType: ChargeTypeOneTime, PeriodUnit: PeriodUnitOnce, PeriodCount: 1, PricePhase: PricePhaseNewPurchase, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1}})
	if err != nil {
		t.Fatal(err)
	}
	targets := fixedPriceTestTargets{"token2": "cpd_22222222222222222222222222222222", "token4": "cpd_44444444444444444444444444444444"}
	for token, dimension := range targets {
		if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code) VALUES(?,?,1,'cpu',?,'CPU','CPU',?,?,'fixed','active')`, dimension, version, token, token, token); err != nil {
			t.Fatal(err)
		}
	}
	rows := []ConfigurationPriceBatchRow{}
	for token := range targets {
		rows = append(rows, ConfigurationPriceBatchRow{CandidateToken: token, Schedule: PriceScheduleCreateInput{BillingRuleNo: rule.BillingRuleNo, CurrencyCode: "CNY", Active: true, Tiers: []PriceTierDefinition{{MinimumQuantity: 0, AmountMinor: 500}}}})
	}
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, targets); err != nil {
		t.Fatal(err)
	}
	for index := range rows {
		anchor, readErr := db.GetValue(ctx, `SELECT price_no FROM plugin_idc_core_catalog_price WHERE configuration_price_dimension_no=?`, targets[rows[index].CandidateToken])
		if readErr != nil {
			t.Fatal(readErr)
		}
		rows[index].PreviousPriceNo = anchor.String()
		rows[index].Schedule.Tiers[0].AmountMinor = 300
	}
	assertUnchanged := func() {
		t.Helper()
		sum, readErr := db.GetValue(ctx, `SELECT SUM(amount_minor) FROM plugin_idc_core_catalog_price WHERE version_no=?`, version)
		if readErr != nil || sum.Int64() != 1000 {
			t.Fatalf("batch leaked partial changes: %v %v", sum, readErr)
		}
	}
	audit.fail = true
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, targets); err == nil {
		t.Fatal("audit failure accepted")
	}
	assertUnchanged()
	audit.fail = false
	rows[1].PreviousPriceNo = "prc_99999999999999999999999999999999"
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, targets); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("stale row accepted: %v", err)
	}
	assertUnchanged()
	anchor, err := db.GetValue(ctx, `SELECT price_no FROM plugin_idc_core_catalog_price WHERE configuration_price_dimension_no=?`, targets[rows[1].CandidateToken])
	if err != nil {
		t.Fatal(err)
	}
	rows[1].PreviousPriceNo = anchor.String()
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, targets); err != nil {
		t.Fatal(err)
	}
	sum, err := db.GetValue(ctx, `SELECT SUM(amount_minor) FROM plugin_idc_core_catalog_price WHERE version_no=?`, version)
	if err != nil || sum.Int64() != 600 {
		t.Fatalf("batch prices not committed: %v %v", sum, err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_configuration_price_dimension SET candidate_value='2' WHERE dimension_no=?`, targets["token2"]); err != nil {
		t.Fatal(err)
	}
	const quantityDimension = "cpd_66666666666666666666666666666666"
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code) VALUES(?,?,1,'cpu','quantity','CPU','CPU','CPU','CPU','per_unit','active')`, quantityDimension, version); err != nil {
		t.Fatal(err)
	}
	quantityRows := []ConfigurationPriceBatchRow{{CandidateToken: "quantity", PricingMode: ConfigurationPricePerUnit, Schedule: PriceScheduleCreateInput{
		BillingRuleNo: rule.BillingRuleNo, CurrencyCode: "CNY", Active: true,
		Tiers: []PriceTierDefinition{{MinimumQuantity: 0, MaximumQuantity: int64Pointer(10), AmountMinor: 100}, {MinimumQuantity: 10, AmountMinor: 50}},
	}}}
	quantityTargets := fixedPriceTestTargets{"quantity": quantityDimension}
	if err = svc.SaveConfigurationPriceBatch(ctx, version, quantityRows, quantityTargets); err != nil {
		t.Fatalf("declared quantity bands not committed: %v", err)
	}
	count, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_price WHERE version_no=? AND configuration_price_dimension_no=?`, version, quantityDimension)
	if err != nil || count.Int() != 2 {
		t.Fatalf("quantity bands incomplete: %v %v", count, err)
	}
	integer := "2"
	selections, _, err := loadSelectedConfigurationPriceDimensions(ctx, db, version, []configurationcontract.ConfigurationValue{{FieldKey: "cpu", IntegerTextValue: &integer}}, `{"cpu":{"numericPricing":"per_choice"}}`)
	if err != nil || len(selections) != 1 || selections[0].DimensionNo != targets["token2"] || selections[0].Quantity != 1 {
		t.Fatalf("specification price mixed with old quantity price: %+v %v", selections, err)
	}
	selections, _, err = loadSelectedConfigurationPriceDimensions(ctx, db, version, []configurationcontract.ConfigurationValue{{FieldKey: "cpu", IntegerTextValue: &integer}}, `{"cpu":{"numericPricing":"quantity"}}`)
	if err != nil || len(selections) != 1 || selections[0].DimensionNo != quantityDimension || selections[0].Quantity != 2 {
		t.Fatalf("quantity price mixed with old specification price: %+v %v", selections, err)
	}

	// Imported configuration and its surcharges are supplier-owned. The same
	// staff actor may still maintain the local base price, without option edits.
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_import_contract(tenant_id,version_no,interface_instance_no,snapshot,snapshot_digest,read_evidence) VALUES(0,?,'pfi_test','{}',repeat('a',64),'{}')`, version); err != nil {
		t.Fatal(err)
	}
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, targets); !bizerr.Is(err, CodeUpstreamConfigurationLocked) {
		t.Fatalf("imported option batch accepted: %v", err)
	}
	optionAnchor, err := db.GetValue(ctx, `SELECT price_no FROM plugin_idc_core_catalog_price WHERE version_no=? AND configuration_price_dimension_no=?`, version, targets["token2"])
	if err != nil {
		t.Fatal(err)
	}
	optionInput := PriceScheduleCreateInput{VersionNo: version, BillingRuleNo: rule.BillingRuleNo, ConfigurationPriceDimensionNo: targets["token2"], CurrencyCode: "CNY", Active: true, Tiers: []PriceTierDefinition{{AmountMinor: 999}}}
	if _, err = svc.CreatePriceSchedule(ctx, optionInput); !bizerr.Is(err, CodeUpstreamConfigurationLocked) {
		t.Fatalf("imported option schedule creation accepted: %v", err)
	}
	if _, err = svc.UpdatePriceSchedule(ctx, optionAnchor.String(), optionInput); !bizerr.Is(err, CodeUpstreamConfigurationLocked) {
		t.Fatalf("imported option schedule update accepted: %v", err)
	}
	if err = svc.DeletePriceSchedule(ctx, version, optionAnchor.String()); !bizerr.Is(err, CodeUpstreamConfigurationLocked) {
		t.Fatalf("imported option schedule delete accepted: %v", err)
	}
	retained, err := db.GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE price_no=?`, optionAnchor.String())
	if err != nil || retained.Int64() != 300 {
		t.Fatalf("rejected imported writes changed source surcharge: %v %v", retained, err)
	}
	baseInput := PriceScheduleCreateInput{VersionNo: version, BillingRuleNo: rule.BillingRuleNo, CurrencyCode: "CNY", Active: true, Tiers: []PriceTierDefinition{{AmountMinor: 1000}}}
	base, err := svc.CreatePriceSchedule(ctx, baseInput)
	if err != nil {
		t.Fatalf("local base price creation rejected: %v", err)
	}
	baseInput.Tiers[0].AmountMinor = 1200
	if _, err = svc.UpdatePriceSchedule(ctx, base.PriceNumbers[0], baseInput); err != nil {
		t.Fatalf("local base price update rejected: %v", err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published' WHERE version_no=?`, version); err != nil {
		t.Fatal(err)
	}
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, targets); err == nil {
		t.Fatal("published batch accepted")
	}
}
