package commerce

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestGenericProductOrderCostPostgreSQL(t *testing.T) {
	t.Run("phone freezes product version cost without resource pool", func(t *testing.T) {
		ctx, database := newCommerceServiceTestDatabase(t)
		seedCommerceServiceCatalog(t, ctx, database)
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_product_version_cost
 (cost_no,version_no,currency_code,period_unit_code,period_count,unit_cost_minor,effective_from,version)
VALUES ('cst_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        'CNY','once',1,420000,CURRENT_TIMESTAMP-INTERVAL '1 minute',3)`); err != nil {
			t.Fatalf("seed generic phone cost: %v", err)
		}
		assertGenericCostTrigger(t, ctx, database)
		checkout := checkoutFixedGenericProduct(t, ctx, database, "generic-phone", 2, "checkout-phone-cost-0001")
		var cost genericOrderCostEvidence
		if err := database.GetScan(ctx, &cost, genericOrderCostEvidenceSQL, checkout.OrderNo); err != nil {
			t.Fatalf("load phone cost: %v", err)
		}
		if cost.StatusCode != "frozen" || cost.VersionNo != "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
			cost.Quantity != 2 || cost.PurchaseCycles != 1 || cost.UnitCostMinor != 420000 ||
			cost.TotalCostMinor != 840000 || cost.SourceCode != "product_version_setting" ||
			cost.ProductVersionCostVersion != 3 {
			t.Fatalf("generic phone cost evidence=%+v", cost)
		}
	})

	t.Run("ssl without a configured cost is explicitly missing", func(t *testing.T) {
		ctx, database := newCommerceServiceTestDatabase(t)
		seedCommerceServiceCatalog(t, ctx, database)
		seedFixedGenericSSL(t, ctx, database)
		checkout := checkoutFixedGenericProduct(t, ctx, database, "generic-ssl", 3, "checkout-ssl-cost-0001")
		var cost genericOrderCostEvidence
		if err := database.GetScan(ctx, &cost, genericOrderCostEvidenceSQL, checkout.OrderNo); err != nil {
			t.Fatalf("load ssl cost: %v", err)
		}
		if cost.StatusCode != "missing" || cost.VersionNo != "ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
			cost.Quantity != 3 || cost.PurchaseCycles != 1 || cost.UnitCostMinor != 0 ||
			cost.TotalCostMinor != 0 || cost.SourceCode != "" {
			t.Fatalf("generic ssl missing cost evidence=%+v", cost)
		}
	})
}

func assertGenericCostTrigger(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	var triggers struct {
		Legacy  int `orm:"legacy"`
		Generic int `orm:"generic"`
	}
	if err := database.GetScan(ctx, &triggers, `SELECT
 count(*) FILTER (WHERE tgrelid='plugin_idc_core_commerce_order_item'::regclass) legacy,
 count(*) FILTER (WHERE tgrelid='plugin_idc_core_commerce_order_item_quantity_snapshot'::regclass) generic
FROM pg_trigger WHERE tgname='trg_idc_core_capture_order_cost' AND NOT tgisinternal`); err != nil {
		t.Fatalf("inspect generic cost trigger: %v", err)
	}
	if triggers.Legacy != 0 || triggers.Generic != 1 {
		t.Fatalf("cost trigger placement=%+v", triggers)
	}
}

type genericOrderCostEvidence struct {
	StatusCode                string `orm:"status_code"`
	VersionNo                 string `orm:"version_no"`
	Quantity                  int    `orm:"quantity"`
	PurchaseCycles            int    `orm:"purchase_cycles"`
	UnitCostMinor             int64  `orm:"unit_cost_minor"`
	TotalCostMinor            int64  `orm:"total_cost_minor"`
	SourceCode                string `orm:"source_code"`
	ProductVersionCostVersion int64  `orm:"product_version_cost_version"`
}

const genericOrderCostEvidenceSQL = `
SELECT status_code,version_no,quantity,purchase_cycles,
       COALESCE(unit_cost_minor,0) unit_cost_minor,
       COALESCE(total_cost_minor,0) total_cost_minor,
       COALESCE(source_code,'') source_code,
       COALESCE(product_version_cost_version,0) product_version_cost_version
FROM plugin_idc_core_commerce_order_cost_snapshot WHERE order_no=?`

func checkoutFixedGenericProduct(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	publicSlug string,
	quantity int,
	idempotencyKey string,
) *Checkout {
	t.Helper()
	reader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, reader)
	configuration, err := reader.GetPurchaseConfiguration(ctx, publicSlug)
	if err != nil {
		t.Fatal(err)
	}
	quoted, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: publicSlug, ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil || quoted == nil || len(quoted.Terms) != 1 {
		t.Fatalf("quote fixed product result=%+v err=%v", quoted, err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	purchase := catalog.PurchaseConfigurationInput{
		PublicSlug: publicSlug, ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quoted.Terms[0].BillingTermToken,
		CurrencyCode:     "CNY", PurchaseCycles: 1,
	}
	service, finalQuote := newCheckoutCustomer(
		t, ctx, database, resolver, purchase, "customer-commerce-service-1", 101, quantity, true,
	)
	checkout, err := service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: finalQuote.QuoteNo, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatalf("checkout fixed product: %v", err)
	}
	return checkout
}

func seedFixedGenericSSL(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_category(category_no,status_code,name_zh_cn,name_en_us)
VALUES ('ctg_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','active','证书','Certificates');
INSERT INTO plugin_idc_core_catalog_product
 (product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us)
VALUES ('prd_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','ctg_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        'active','generic-ssl','SSL 证书','SSL Certificate');
INSERT INTO plugin_idc_core_catalog_product_version
 (version_no,product_no,version_number,status_code,source_code,published_at)
VALUES ('ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','prd_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
		1,'draft','manual',NULL);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
 (version_no,strategy_code,granularity_code,ongoing_management,maximum_attempts,
  retry_base_seconds,policy_digest,created_by)
VALUES ('ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','manual','per_order_item',FALSE,10,30,repeat('d',64),1);
UPDATE plugin_idc_core_catalog_product_version
SET status_code='published',published_at=CURRENT_TIMESTAMP
WHERE version_no='ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
UPDATE plugin_idc_core_catalog_product
SET current_version_no='ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
WHERE product_no='prd_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
INSERT INTO plugin_idc_core_catalog_billing_rule
 (billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,status_code)
VALUES ('bil_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        'once','one_time','once',1,'new_purchase','active');
INSERT INTO plugin_idc_core_catalog_price
 (price_no,version_no,billing_rule_no,option_no,currency_code,amount_minor,setup_fee_minor,renewal_amount_minor,status_code)
VALUES ('prc_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        'bil_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',NULL,'CNY',9900,0,NULL,'active');
INSERT INTO plugin_idc_core_catalog_tax_policy
 (tax_policy_no,version_no,currency_code,price_includes_tax,tax_treatment_code,
  tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor)
VALUES ('tax_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        'CNY',TRUE,'exempt',NULL,'digital_service','line','half_up',1);
INSERT INTO plugin_idc_core_catalog_product_version_inventory
 (version_no,tracking_enabled,sellable_quantity,version)
VALUES ('ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',FALSE,0,1)`); err != nil {
		t.Fatalf("seed generic ssl: %v", err)
	}
}
