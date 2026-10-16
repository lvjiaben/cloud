// This file verifies that renewal follows the effective paid price snapshot,
// while the original entitlement and order retain their purchase identities.

package renewal

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestRenewalLoadsEffectivePaidPeriodPostgreSQL(t *testing.T) {
	ctx, database := newRenewalPostgreSQLDatabase(t)
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Connection-local tables exercise the production join independently of
		// external payment fixtures. They do not claim a completed conversion.
		if _, err := database.Exec(ctx, `
CREATE TEMP TABLE plugin_idc_core_customer_subject ON COMMIT DROP AS SELECT
 0 AS tenant_id,'CUS_owner'::text AS customer_no,'active'::text AS status_code,1::bigint AS version;
CREATE TEMP TABLE plugin_idc_core_commerce_order_item ON COMMIT DROP AS SELECT
 0 AS tenant_id,'item'::text AS order_item_no,'order'::text AS order_no,'CFG_original'::text AS configuration_snapshot_no;
CREATE TEMP TABLE plugin_idc_core_service_entitlement ON COMMIT DROP AS SELECT
 0 AS tenant_id,'ENT_owned'::text AS entitlement_no,'CUS_owner'::text AS customer_no,
 'product'::text AS product_no,'original-version'::text AS version_no,'original-monthly'::text AS billing_rule_no,
 'SCF_original'::text AS configuration_snapshot_no,'CNY'::text AS currency_code,'recurring'::text AS charge_type_code,'active'::text AS status_code,
 'local_entitlement'::text AS renewal_mode_code,''::text AS renewal_method,'owner'::text AS entitlement_owner_id,
 'connector'::text AS plugin_id,'1.0.0'::text AS plugin_release_version,'v1'::text AS contract_version,
 repeat('a',64) AS declaration_digest,'month'::text AS period_unit_code,1 AS period_count,
 CURRENT_TIMESTAMP+INTERVAL '1 month' AS expires_at,2::bigint AS version,
 'order'::text AS order_no,'item'::text AS order_item_no,NULL::text AS current_price_snapshot_no;
CREATE TEMP TABLE plugin_idc_core_service_price_snapshot ON COMMIT DROP AS SELECT
 0 AS tenant_id,'RPS_effective'::text AS price_snapshot_no,'entitlement'::text AS subject_type_code,
 'ENT_owned'::text AS subject_no,'current-version'::text AS version_no,'current-annual'::text AS billing_rule_no,
 'CFG_effective'::text AS configuration_snapshot_no,'year'::text AS period_unit_code,2 AS period_count;
`); err != nil {
			return err
		}
		for _, hasEffectivePrice := range []bool{false, true} {
			if hasEffectivePrice {
				if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET current_price_snapshot_no='RPS_effective'`); err != nil {
					return err
				}
			}
			row, err := lockOwnedEntitlementForQuote(ctx, database, "CUS_owner", "ENT_owned")
			if err != nil {
				return err
			}
			wantVersion, wantRule, wantConfiguration, wantUnit, wantCount := "original-version", "original-monthly", "CFG_original", "month", 1
			if hasEffectivePrice {
				wantVersion, wantRule, wantConfiguration, wantUnit, wantCount = "current-version", "current-annual", "CFG_effective", "year", 2
			}
			if row == nil || row.ChargeTypeCode != "recurring" || row.VersionNo != wantVersion || row.BillingRuleNo != wantRule ||
				row.PriceConfigurationSnapshotNo != wantConfiguration || row.PeriodUnitCode != wantUnit || row.PeriodCount != wantCount ||
				row.ConfigurationSnapshotNo != "SCF_original" {
				t.Fatalf("effective=%v renewal facts=%+v", hasEffectivePrice, row)
			}
			if foreign, err := lockOwnedEntitlementForQuote(ctx, database, "CUS_other", "ENT_owned"); err != nil || foreign != nil {
				t.Fatalf("effective=%v cross-customer facts=%+v err=%v", hasEffectivePrice, foreign, err)
			}
		}
		original, err := database.GetOne(ctx, `SELECT version_no,billing_rule_no,period_unit_code,period_count FROM plugin_idc_core_service_entitlement`)
		if err != nil {
			return err
		}
		if original["version_no"].String() != "original-version" || original["billing_rule_no"].String() != "original-monthly" ||
			original["period_unit_code"].String() != "month" || original["period_count"].Int() != 1 {
			t.Fatalf("renewal read rewrote original purchase facts: %v", original)
		}
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='trial'`); err != nil {
			return err
		}
		service := &serviceImpl{database: database, principal: &renewalPostgreSQLPrincipal{audience: &principal.Audience{
			Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "CUS_owner", UserID: 1,
		}}}
		input := QuoteInput{EntitlementNo: "ENT_owned", RenewalCycles: 1, IdempotencyKey: "trial-ordinary-renewal"}
		// Nil pricing, marketing and payment dependencies ensure admission happens
		// before any ordinary quote or financial execution can start.
		for _, quote := range []func() (*Quote, error){
			func() (*Quote, error) { return service.QuoteCurrentCustomer(ctx, input) },
			func() (*Quote, error) { return service.QuoteForStaff(ctx, "CUS_owner", input) },
			func() (*Quote, error) { return service.QuoteForChannel(ctx, "CUS_owner", input) },
		} {
			result, err := quote()
			if !bizerr.Is(err, CodeCapabilityUnavailable) || result != nil {
				t.Fatalf("trial entered ordinary renewal: quote=%+v err=%v", result, err)
			}
		}
		for _, table := range []string{
			"plugin_idc_core_service_renewal_quote_request", "plugin_idc_core_commerce_quote_snapshot",
			"plugin_idc_core_business_operation", "plugin_idc_core_billing_invoice", "plugin_idc_core_billing_ledger_transaction",
		} {
			count, err := database.GetValue(ctx, "SELECT COUNT(*) FROM "+table)
			if err != nil || count.Int() != 0 {
				t.Fatalf("rejected trial wrote %s: count=%v err=%v", table, count, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
