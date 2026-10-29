// This file verifies complete draft schedule replacement, stale fences,
// authorization, rollback and published-version protection in isolated PostgreSQL.
package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type scheduleMutationAudit struct {
	idccap.BusinessAuditWriter
	fail bool
}

func (a *scheduleMutationAudit) AppendBusinessTransition(_ context.Context, _ idccap.BusinessAuditTransition) error {
	if a.fail {
		return errors.New("audit unavailable")
	}
	return nil
}

func TestDraftPriceScheduleMaintenancePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	for _, table := range []string{"catalog_product_version", "catalog_billing_rule", "catalog_price", "catalog_configuration_price_dimension"} {
		if _, err := db.Exec(ctx, "CREATE TABLE plugin_idc_core_"+table+" (LIKE public.plugin_idc_core_"+table+" INCLUDING DEFAULTS INCLUDING IDENTITY)"); err != nil {
			t.Fatal(err)
		}
	}
	const version = "ver_11111111111111111111111111111111"
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code,source_version,source_fingerprint,created_by) VALUES(?,'prd_test',1,'draft','manual','1',repeat('a',64),81)`, version); err != nil {
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
	input := PriceScheduleCreateInput{VersionNo: version, BillingRuleNo: rule.BillingRuleNo, CurrencyCode: "CNY", Active: true, Tiers: []PriceTierDefinition{{MinimumQuantity: 0, AmountMinor: 1000}}}
	original, err := svc.CreatePriceSchedule(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Tiers[0].AmountMinor = 2000
	updated, err := svc.UpdatePriceSchedule(ctx, original.PriceNumbers[0], input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PriceNumbers[0] == original.PriceNumbers[0] || updated.Tiers[0].AmountMinor != 2000 {
		t.Fatalf("replacement lost row fence or amount: %+v", updated)
	}
	if _, err = svc.UpdatePriceSchedule(ctx, original.PriceNumbers[0], input); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("stale update accepted: %v", err)
	}
	audit.fail = true
	if err = svc.DeletePriceSchedule(ctx, version, updated.PriceNumbers[0]); err == nil {
		t.Fatal("audit failure accepted")
	}
	remaining, err := db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_catalog_price WHERE version_no=?", version)
	if err != nil || remaining.Int() != 1 {
		t.Fatalf("delete rollback: %v %v", remaining, err)
	}
	if _, err = svc.UpdatePriceSchedule(ctx, updated.PriceNumbers[0], input); err == nil {
		t.Fatal("replacement audit failure accepted")
	}
	remaining, err = db.GetValue(ctx, "SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE price_no=?", updated.PriceNumbers[0])
	if err != nil || remaining.Int64() != 2000 {
		t.Fatalf("replacement rollback lost old schedule: %v %v", remaining, err)
	}
	audit.fail = false
	if _, err = db.Exec(ctx, "UPDATE plugin_idc_core_catalog_product_version SET status_code='published' WHERE version_no=?", version); err != nil {
		t.Fatal(err)
	}
	if err = svc.DeletePriceSchedule(ctx, version, updated.PriceNumbers[0]); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("published delete: %v", err)
	}
	if _, err = db.Exec(ctx, "UPDATE plugin_idc_core_catalog_product_version SET status_code='draft' WHERE version_no=?", version); err != nil {
		t.Fatal(err)
	}
	if err = svc.DeletePriceSchedule(ctx, version, updated.PriceNumbers[0]); err != nil {
		t.Fatal(err)
	}
	remaining, err = db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_catalog_price WHERE version_no=?", version)
	if err != nil || remaining.Int() != 0 {
		t.Fatalf("schedule not removed: %v %v", remaining, err)
	}
	// A second schedule verifies that complete replacement/deletion includes
	// every tier, regardless of which row anchors the mutation.
	tierRule, err := svc.CreateBillingRule(ctx, BillingRuleCreateInput{VersionNo: version, RuleCode: "tiered", Active: true, Definition: BillingRuleDefinition{ChargeType: ChargeTypeTiered, PeriodUnit: PeriodUnitMonth, PeriodCount: 1, PricePhase: PricePhaseNewPurchase, QuantityMode: QuantityModeTiered, TierMode: "graduated", MinimumPurchaseCycles: 1}})
	if err != nil {
		t.Fatal(err)
	}
	boundary := int64(10)
	tierInput := PriceScheduleCreateInput{VersionNo: version, BillingRuleNo: tierRule.BillingRuleNo, CurrencyCode: "CNY", Active: true, Tiers: []PriceTierDefinition{{MinimumQuantity: 0, MaximumQuantity: &boundary, AmountMinor: 100}, {MinimumQuantity: 10, AmountMinor: 50}}}
	tiered, err := svc.CreatePriceSchedule(ctx, tierInput)
	if err != nil {
		t.Fatal(err)
	}
	tierInput.Tiers[1].AmountMinor = 80
	replaced, err := svc.UpdatePriceSchedule(ctx, tiered.PriceNumbers[1], tierInput)
	if err != nil || len(replaced.PriceNumbers) != 2 {
		t.Fatalf("replace tiers: %+v %v", replaced, err)
	}
	remaining, err = db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_catalog_price WHERE billing_rule_no=?", tierRule.BillingRuleNo)
	if err != nil || remaining.Int() != 2 {
		t.Fatalf("old tiers survived replacement: %v %v", remaining, err)
	}
	if err = svc.DeletePriceSchedule(ctx, version, replaced.PriceNumbers[1]); err != nil {
		t.Fatal(err)
	}
	remaining, err = db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_catalog_price WHERE billing_rule_no=?", tierRule.BillingRuleNo)
	if err != nil || remaining.Int() != 0 {
		t.Fatalf("partial tier deletion: %v %v", remaining, err)
	}

}
