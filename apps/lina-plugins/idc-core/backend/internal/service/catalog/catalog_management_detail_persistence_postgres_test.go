// This file verifies detail projections against real PostgreSQL in a disposable
// schema; product and version state/timestamps must never overwrite each other.
package catalog

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestCatalogManagementDetailSeparateStatePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	for _, table := range []string{"catalog_category", "catalog_product", "catalog_product_version", "catalog_product_version_inventory", "catalog_billing_rule", "catalog_price", "catalog_configuration_price_dimension", "catalog_tax_policy", "catalog_product_version_fulfillment_policy", "product_directory_membership"} {
		if _, err := db.Exec(ctx, "CREATE TABLE plugin_idc_core_"+table+" (LIKE public.plugin_idc_core_"+table+" INCLUDING DEFAULTS INCLUDING IDENTITY)"); err != nil {
			t.Fatal(err)
		}
	}
	fixture := []string{
		`INSERT INTO plugin_idc_core_catalog_category(category_no,name_zh_cn,name_en_us,created_by,updated_by) VALUES('category_detail_test','Detail category','Detail category',81,81)`,
		`INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us,created_by,updated_by,created_at,updated_at) VALUES('product_detail_test','category_detail_test','active','detail-test','Detail product','Detail product',81,81,'2026-01-01T00:00:00Z','2026-02-01T00:00:00Z')`,
		`INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code,source_version,source_fingerprint,created_by,created_at) VALUES('version_detail_test','product_detail_test',1,'draft','manual','1',repeat('a',64),81,'2026-03-01T00:00:00Z')`,
		`INSERT INTO plugin_idc_core_catalog_product_version_inventory(version_no) VALUES('version_detail_test')`,
		`INSERT INTO plugin_idc_core_catalog_billing_rule(billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,quantity_mode_code,status_code) VALUES('billing_detail_test','version_detail_test','monthly','recurring','month',1,'both','fixed','active')`,
		`INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code) VALUES('dimension_detail_test','version_detail_test',1,'cpu','quantity','处理器','CPU','处理器','CPU','per_unit')`,
		`INSERT INTO plugin_idc_core_catalog_price(price_no,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,amount_minor,minimum_quantity,status_code) VALUES('price_detail_test','version_detail_test','billing_detail_test','dimension_detail_test','CNY',500,0,'active')`,
	}
	for _, sql := range fixture {
		if _, err := db.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	detail, err := loadProductManagementDetail(ctx, "product_detail_test")
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil {
		t.Fatal("detail missing")
	}
	if detail.Product.Status != "active" || detail.Version.Status != "draft" {
		t.Fatalf("status collision: product=%s version=%s", detail.Product.Status, detail.Version.Status)
	}
	if len(detail.Prices) != 1 || detail.Prices[0].ConfigurationPricingMode != ConfigurationPricePerUnit {
		t.Fatalf("saved local pricing mode missing: %+v", detail.Prices)
	}
	productCreated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	versionCreated := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !detail.Product.CreatedAt.Equal(productCreated) || !detail.Version.CreatedAt.Equal(versionCreated) || !detail.Product.UpdatedAt.Equal(updated) {
		t.Fatalf("timestamp collision: product=%s updated=%s version=%s", detail.Product.CreatedAt, detail.Product.UpdatedAt, detail.Version.CreatedAt)
	}
}
