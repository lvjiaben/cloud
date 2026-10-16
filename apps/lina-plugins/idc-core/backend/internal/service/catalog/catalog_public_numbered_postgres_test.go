// This file verifies that supplier pickers count only published, completely
// priced public products and retain the exact total on an empty numeric page.

package catalog

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPublicCatalogNumberedPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, publicNumberedFixtureSQL); err != nil {
		t.Fatal(err)
	}
	reader, err := NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		page int
		slug string
	}{{1, "alpha-one"}, {2, "alpha-two"}, {3, ""}} {
		page, queryErr := reader.SearchNumbered(ctx, PublicCatalogNumberedInput{Page: tc.page, PageSize: 1, NamePrefix: "Alpha"})
		if queryErr != nil || page.Total != 2 {
			t.Fatalf("page %d = %+v err=%v", tc.page, page, queryErr)
		}
		if tc.slug == "" {
			if len(page.Products) != 0 {
				t.Fatal("out-of-range page returned products")
			}
		} else if len(page.Products) != 1 || page.Products[0].PublicSlug != tc.slug || len(page.Products[0].Currencies) != 1 || page.Products[0].Currencies[0] != "CNY" {
			t.Fatalf("unexpected numbered product: %+v", page.Products)
		}
	}
	page, err := reader.SearchNumbered(ctx, PublicCatalogNumberedInput{Page: 1, PageSize: 100, Currency: "USD"})
	if err != nil || page.Total != 0 || len(page.Products) != 0 {
		t.Fatalf("unavailable currency leaked products: %+v err=%v", page, err)
	}
	if _, err = reader.SearchNumbered(ctx, PublicCatalogNumberedInput{Page: 1, PageSize: 101}); err == nil {
		t.Fatal("unbounded page accepted")
	}
	// An incompletely priced product sorts first. It must be filtered before
	// the cursor limit, otherwise a short page hides later saleable products.
	cursorPage, err := reader.Search(ctx, PublicCatalogSearchInput{Limit: 1, NamePrefix: "Alpha"})
	if err != nil || len(cursorPage.Products) != 1 || cursorPage.Products[0].PublicSlug != "alpha-one" || cursorPage.NextCursor == "" {
		t.Fatalf("cursor filter did not run before paging: %+v err=%v", cursorPage, err)
	}
}

// TestPublicCatalogCurrencyAdmissionPostgreSQL proves removal is visible before
// pagination across the anonymous cursor, numeric picker and product detail.
func TestPublicCatalogCurrencyAdmissionPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, publicNumberedFixtureSQL); err != nil {
		t.Fatal(err)
	}
	reader, err := NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_merchant_currency VALUES(0,'EUR',TRUE,FALSE);
 INSERT INTO plugin_idc_core_catalog_price (tenant_id,version_no,billing_rule_no,currency_code,option_no,status_code,configuration_price_dimension_no)
 SELECT tenant_id,version_no,billing_rule_no,'EUR',NULL,'active','configuration-only' FROM plugin_idc_core_catalog_billing_rule;`); err != nil {
		t.Fatal(err)
	}
	configOnly, err := reader.Search(ctx, PublicCatalogSearchInput{Limit: 1, Currency: "EUR"})
	if err != nil || len(configOnly.Products) != 0 {
		t.Fatalf("configuration-only prices became a base: %+v err=%v", configOnly, err)
	}
	configNumbered, err := reader.SearchNumbered(ctx, PublicCatalogNumberedInput{Page: 1, PageSize: 1, Currency: "EUR"})
	if err != nil || configNumbered.Total != 0 {
		t.Fatalf("configuration-only numeric total: %+v err=%v", configNumbered, err)
	}

	// Give both products a complete USD price. CNY and historical prices remain
	// intact when admission changes, including an enabled-but-deleted currency.
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_price (tenant_id,version_no,billing_rule_no,currency_code,option_no,status_code) SELECT tenant_id,version_no,billing_rule_no,'USD',NULL,'active' FROM plugin_idc_core_catalog_billing_rule WHERE version_no IN ('v1','v2');`); err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct {
		name             string
		enabled, deleted bool
		visible          bool
	}{
		{"enabled", true, false, true}, {"disabled", false, false, false},
		{"deleted", true, true, false}, {"restored", true, false, true},
	} {
		t.Run(state.name, func(t *testing.T) {
			if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_merchant_currency SET enabled=?,deleted=? WHERE tenant_id=0 AND currency_code='USD'`, state.enabled, state.deleted); err != nil {
				t.Fatal(err)
			}
			page, err := reader.Search(ctx, PublicCatalogSearchInput{Limit: 1, NamePrefix: "Alpha", Currency: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if state.visible {
				want = 1
			}
			if len(page.Products) != want {
				t.Fatalf("cursor product count=%d want=%d", len(page.Products), want)
			}
			expectedCurrencies := []string{"CNY"}
			if state.visible {
				expectedCurrencies = append(expectedCurrencies, "USD")
			}
			if !reflect.DeepEqual(page.AvailableCurrencies, expectedCurrencies) {
				t.Fatalf("choices=%v want=%v", page.AvailableCurrencies, expectedCurrencies)
			}
			numbered, err := reader.SearchNumbered(ctx, PublicCatalogNumberedInput{Page: 1, PageSize: 1, NamePrefix: "Alpha", Currency: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			total := 0
			if state.visible {
				total = 2
			}
			if numbered.Total != total || len(numbered.Products) != want {
				t.Fatalf("numeric page=%+v want total=%d", numbered, total)
			}
			detail, err := reader.GetBySlug(ctx, "alpha-one")
			if err != nil || detail == nil || !reflect.DeepEqual(detail.Currencies, expectedCurrencies) {
				t.Fatalf("detail=%+v err=%v", detail, err)
			}
			// No admission change can erase the stored price used by historical money.
			count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_price WHERE version_no IN ('v1','v2') AND currency_code='USD'`)
			if err != nil || count.Int() != 2 {
				t.Fatalf("historical prices changed: count=%v err=%v", count, err)
			}
		})
	}
	// A product with no remaining enabled price must disappear before paging,
	// including its detail, and must not leave a stale numeric total.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_merchant_currency SET enabled=FALSE`); err != nil {
		t.Fatal(err)
	}
	page, err := reader.Search(ctx, PublicCatalogSearchInput{Limit: 1})
	if err != nil || len(page.Products) != 0 || len(page.AvailableCurrencies) != 0 || page.NextCursor != "" {
		t.Fatalf("no enabled currency page=%+v err=%v", page, err)
	}
	numbered, err := reader.SearchNumbered(ctx, PublicCatalogNumberedInput{Page: 1, PageSize: 1})
	if err != nil || numbered.Total != 0 || len(numbered.Products) != 0 {
		t.Fatalf("no enabled currency numeric=%+v err=%v", numbered, err)
	}
	if _, err = reader.GetBySlug(ctx, "alpha-one"); err == nil {
		t.Fatal("unbuyable detail remained visible")
	}
}

const publicNumberedFixtureSQL = `
CREATE TABLE plugin_idc_core_merchant_currency (tenant_id int,currency_code char(3),enabled bool,deleted bool,PRIMARY KEY(tenant_id,currency_code));
INSERT INTO plugin_idc_core_merchant_currency VALUES(0,'CNY',TRUE,FALSE),(0,'USD',TRUE,FALSE);
CREATE TABLE plugin_idc_core_catalog_category (id bigint,tenant_id int,category_no text,name_zh_cn text,name_en_us text,status_code text,sort_order int,deleted_at timestamp);
CREATE TABLE plugin_idc_core_catalog_product (id bigint,tenant_id int,product_no text,category_no text,current_version_no text,public_slug text,name_zh_cn text,name_en_us text,summary_zh_cn text,summary_en_us text,status_code text,sort_order int,deleted_at timestamp);
CREATE TABLE plugin_idc_core_catalog_product_version (tenant_id int,product_no text,version_no text,status_code text);
CREATE TABLE plugin_idc_core_catalog_product_version_inventory (version_no text,tracking_enabled bool,sellable_quantity int);
CREATE TABLE plugin_idc_core_commerce_inventory_reservation (version_no text,reserved_quantity int,status_code text);
CREATE TABLE plugin_idc_core_catalog_billing_rule (tenant_id int,version_no text,billing_rule_no text,status_code text);
CREATE TABLE plugin_idc_core_catalog_price (tenant_id int,version_no text,billing_rule_no text,currency_code text,option_no text,status_code text,configuration_price_dimension_no text);
INSERT INTO plugin_idc_core_catalog_category VALUES(1,0,'ctg_11111111111111111111111111111111','分类','Category','active',0,NULL);
INSERT INTO plugin_idc_core_catalog_product VALUES
(1,0,'p0','ctg_11111111111111111111111111111111','v0','alpha-no-price','Alpha missing','Alpha missing','','','active',0,NULL),
(2,0,'p1','ctg_11111111111111111111111111111111','v1','alpha-one','Alpha one','Alpha one','','','active',1,NULL),
(3,0,'p2','ctg_11111111111111111111111111111111','v2','alpha-two','Alpha two','Alpha two','','','active',2,NULL),
(4,0,'p3','ctg_11111111111111111111111111111111','v3','alpha-draft','Alpha draft','Alpha draft','','','active',3,NULL),
(5,0,'p4','ctg_11111111111111111111111111111111','v4','alpha-disabled','Alpha disabled','Alpha disabled','','','disabled',4,NULL),
(6,0,'p5','ctg_11111111111111111111111111111111','v5','alpha-partial','Alpha partial','Alpha partial','','','active',0,NULL),
(7,0,'p6','ctg_11111111111111111111111111111111','v6','alpha-mixed','Alpha mixed','Alpha mixed','','','active',0,NULL);
INSERT INTO plugin_idc_core_catalog_product_version VALUES(0,'p0','v0','published'),(0,'p1','v1','published'),(0,'p2','v2','published'),(0,'p3','v3','draft'),(0,'p4','v4','published'),(0,'p5','v5','published'),(0,'p6','v6','published');
INSERT INTO plugin_idc_core_catalog_product_version_inventory SELECT version_no,false,0 FROM plugin_idc_core_catalog_product_version;
INSERT INTO plugin_idc_core_catalog_billing_rule SELECT 0,version_no,'b-'||version_no,'active' FROM plugin_idc_core_catalog_product_version;
INSERT INTO plugin_idc_core_catalog_price (tenant_id,version_no,billing_rule_no,currency_code,option_no,status_code) SELECT 0,version_no,'b-'||version_no,'CNY',NULL,'active' FROM plugin_idc_core_catalog_product_version WHERE version_no<>'v0';
INSERT INTO plugin_idc_core_catalog_billing_rule VALUES(0,'v5','b-v5-year','active'),(0,'v6','b-v6-year','active');
INSERT INTO plugin_idc_core_catalog_price (tenant_id,version_no,billing_rule_no,currency_code,option_no,status_code) VALUES(0,'v6','b-v6-year','USD',NULL,'active');
`
