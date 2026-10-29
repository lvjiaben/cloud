// This file verifies explicit base-price targets, atomic rollback and stale
// schedule fences in a disposable schema without changing installed products.
package catalog

import (
	"context"
	"crypto/rand"
	"os"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestBasePriceBatchAtomicPostgreSQL(t *testing.T) {
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
	rows := []ConfigurationPriceBatchRow{}
	for index, currency := range []string{"CNY", "EUR"} {
		rows = append(rows, ConfigurationPriceBatchRow{TargetKind: PriceBatchTargetBase, Schedule: PriceScheduleCreateInput{BillingRuleNo: rule.BillingRuleNo, CurrencyCode: currency, Active: true, Tiers: []PriceTierDefinition{{MinimumQuantity: 0, AmountMinor: int64(index+1) * 500}}}})
	}
	// Base-only batches need no configuration resolver or configuration access.
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, nil); err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		anchor, e := db.GetValue(ctx, `SELECT price_no FROM plugin_idc_core_catalog_price WHERE version_no=? AND currency_code=?`, version, rows[i].Schedule.CurrencyCode)
		if e != nil {
			t.Fatal(e)
		}
		rows[i].PreviousPriceNo = anchor.String()
		rows[i].Schedule.Tiers[0].AmountMinor = 300
	}
	assertUnchanged := func() {
		t.Helper()
		total, e := db.GetValue(ctx, `SELECT SUM(amount_minor) FROM plugin_idc_core_catalog_price WHERE version_no=?`, version)
		if e != nil || total.Int64() != 1500 {
			t.Fatalf("partial base-price changes survived: %v %v", total, e)
		}
	}
	for _, invalid := range []ConfigurationPriceBatchRow{
		{PreviousPriceNo: rows[0].PreviousPriceNo, Schedule: rows[0].Schedule},
		{TargetKind: PriceBatchTargetBase, CandidateToken: "unexpected-option", PreviousPriceNo: rows[0].PreviousPriceNo, Schedule: rows[0].Schedule},
		{TargetKind: "invalid", PreviousPriceNo: rows[0].PreviousPriceNo, Schedule: rows[0].Schedule},
	} {
		if err = svc.SaveConfigurationPriceBatch(ctx, version, []ConfigurationPriceBatchRow{invalid}, nil); !bizerr.Is(err, CodeManagementInvalid) {
			t.Fatalf("ambiguous target accepted: %v", err)
		}
		assertUnchanged()
	}
	audit.fail = true
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, nil); err == nil {
		t.Fatal("audit failure accepted")
	}
	assertUnchanged()
	audit.fail = false
	anchor := rows[1].PreviousPriceNo
	rows[1].PreviousPriceNo = "prc_99999999999999999999999999999999"
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, nil); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("stale schedule accepted: %v", err)
	}
	assertUnchanged()
	rows[1].PreviousPriceNo = anchor
	// Any invalid price rejects the entire batch before another row can change.
	rows[1].Schedule.Tiers[0].AmountMinor = -1
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, nil); err == nil {
		t.Fatal("negative price accepted")
	}
	assertUnchanged()
	rows[1].Schedule.Tiers[0].AmountMinor = 300
	rows[0].Schedule.Tiers[0].AmountMinor = 0
	rows[1].Schedule.Active = false
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, nil); err != nil {
		t.Fatal(err)
	}
	free, e := db.GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE version_no=? AND currency_code='CNY'`, version)
	if e != nil || free == nil || free.Int64() != 0 {
		t.Fatalf("explicit free base price was lost: %v %v", free, e)
	}
	euros, e := db.GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE version_no=? AND currency_code='EUR'`, version)
	if e != nil || euros.Int64() != 300 {
		t.Fatalf("second currency did not commit together: %v %v", euros, e)
	}
	status, e := db.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_catalog_price WHERE version_no=? AND currency_code='EUR'`, version)
	if e != nil || status.String() != "inactive" {
		t.Fatalf("editing an inactive price enabled it: %v %v", status, e)
	}
	// A response-lost retry keeps its old anchors and cannot write a second time.
	if err = svc.SaveConfigurationPriceBatch(ctx, version, rows, nil); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("old anchors replayed: %v", err)
	}
}
