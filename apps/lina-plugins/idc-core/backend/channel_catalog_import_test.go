// This file verifies that authenticated export appends the current customer's
// effective price without deleting a previously imported resale rule.

package backend

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"os"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

type resaleCatalogStub struct {
	catalog.CatalogReader
	snapshot *channelcommercecap.ImportSnapshot
}

func (s resaleCatalogStub) ReadImportSnapshot(context.Context, string) (*channelcommercecap.ImportSnapshot, error) {
	return s.snapshot, nil
}

type resalePriceStub struct {
	rules []salesmanagement.ResolvedPriceRule
}

func (s resalePriceStub) ResolvePriceRules(context.Context, string, string, time.Time) ([]salesmanagement.ResolvedPriceRule, error) {
	return s.rules, nil
}

func TestChannelImportPreservesUpstreamPriceAndCustomerOverride(t *testing.T) {
	dsn := os.Getenv(catalogIntegrationDSNEnv)
	if dsn == "" {
		t.Skip("PostgreSQL DSN required")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newCatalogIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if _, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_customer_subject(tenant_id int,customer_no text,customer_type_code text,status_code text);
 CREATE TABLE plugin_idc_core_customer_enterprise_profile(tenant_id int,customer_no text,verification_status_code text);
 CREATE TABLE plugin_idc_core_customer_individual_profile(tenant_id int,customer_no text,identity_status_code text);
 CREATE TABLE plugin_idc_core_customer_identity_binding(id int,tenant_id int,customer_no text,sys_user_id int,status_code text);
 INSERT INTO plugin_idc_core_customer_subject VALUES(0,'customer-1','individual','active');
 INSERT INTO plugin_idc_core_customer_individual_profile VALUES(0,'customer-1','verified');
 INSERT INTO plugin_idc_core_customer_identity_binding VALUES(1,0,'customer-1',42,'active');`); err != nil {
		t.Fatal(err)
	}

	for _, kind := range []string{"discount", "fixed"} {
		t.Run(kind, func(t *testing.T) {
			fixed, bps := int64(900), 1000
			prior := &channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed}
			snapshot := &channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportProtocolVersion, ProductNo: "product-1", Product: channelcommercecap.CatalogProduct{PublicSlug: "demo"}, ConfigurationRevision: "r1", Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: "per_order_item", RenewalMode: "unavailable"}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 1000}}, WholeItemRule: prior}}}
			rule := salesmanagement.ResolvedPriceRule{CurrencyCode: "CNY", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, Kind: kind}
			if kind == "discount" {
				rule.DiscountBasisPoints = &bps
			} else {
				rule.FixedAmountMinor = &fixed
			}
			runtime := &channelCommerceRuntime{database: db, catalog: resaleCatalogStub{snapshot: snapshot}, pricing: resalePriceStub{rules: []salesmanagement.ResolvedPriceRule{rule}}}
			result, err := runtime.ReadCatalogImportSnapshot(context.Background(), "customer-1", "demo")
			if err != nil {
				t.Fatal(err)
			}
			if result.Prices[0].WholeItemRule.Prior != prior || result.Prices[0].WholeItemRule.Kind != kind {
				t.Fatalf("upstream rule lost: %+v", result.Prices[0].WholeItemRule)
			}
		})
	}
}
