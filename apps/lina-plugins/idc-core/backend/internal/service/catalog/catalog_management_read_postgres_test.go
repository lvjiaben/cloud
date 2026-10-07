// This file tests exact catalog totals and bounded numeric pages in a disposable
// PostgreSQL schema. It never queries or changes installed merchant products.
package catalog

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestCatalogManagementNumberedPagesPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, catalogNumberedPageFixtureSQL); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 25; index++ {
		productNo, versionNo := fmt.Sprintf("prd_%032x", index), fmt.Sprintf("ver_%032x", index)
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,name_zh_cn,name_en_us,public_slug) VALUES (?,'ctg_11111111111111111111111111111111','商品','Literal_% product',?)`, productNo, productNo); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(product_no,version_no) VALUES (?,?)`, productNo, versionNo); err != nil {
			t.Fatal(err)
		}
	}
	codec := testProductManagementCursorCodec(t)
	for _, scenario := range []struct{ page, size, count int }{{1, 10, 10}, {2, 10, 10}, {3, 10, 5}, {50, 10, 0}, {1, 100, 25}} {
		input, err := normalizeProductManagementSearch(ProductManagementSearchInput{Page: scenario.page, Limit: scenario.size}, codec)
		if err != nil {
			t.Fatal(err)
		}
		result, err := searchProductManagementPage(ctx, input)
		if err != nil || result.Total != 25 || len(result.Products) != scenario.count {
			t.Fatalf("scenario=%+v result=%+v err=%v", scenario, result, err)
		}
		if scenario.count > 0 && result.Products[0].ProductNo != fmt.Sprintf("prd_%032x", (scenario.page-1)*scenario.size+1) {
			t.Fatalf("wrong numeric offset: %+v", result)
		}
	}
	for _, scenario := range []struct {
		name  string
		total int
	}{{"literal_%", 25}, {"missing", 0}} {
		input, err := normalizeProductManagementSearch(ProductManagementSearchInput{NamePrefix: scenario.name, Page: 1, Limit: 10}, codec)
		if err != nil {
			t.Fatal(err)
		}
		result, err := searchProductManagementPage(ctx, input)
		if err != nil || result.Total != scenario.total {
			t.Fatalf("filter result=%+v err=%v", result, err)
		}
	}
	if _, err := g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET tenant_id=9 WHERE product_no='prd_00000000000000000000000000000001'`); err != nil {
		t.Fatal(err)
	}
	input, err := normalizeProductManagementSearch(ProductManagementSearchInput{Page: 1, Limit: 10}, codec)
	if err != nil {
		t.Fatal(err)
	}
	result, err := searchProductManagementPage(ctx, input)
	if err != nil || result.Total != 24 {
		t.Fatalf("tenant isolation result=%+v err=%v", result, err)
	}
}

const catalogNumberedPageFixtureSQL = `
CREATE TABLE plugin_idc_core_catalog_category(tenant_id INT DEFAULT 0, category_no TEXT PRIMARY KEY,name_zh_cn TEXT DEFAULT '',name_en_us TEXT DEFAULT '',deleted_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_catalog_product(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT DEFAULT 0,product_no TEXT,category_no TEXT,sort_order INT DEFAULT 0,status_code TEXT DEFAULT 'active',public_slug TEXT,name_zh_cn TEXT,name_en_us TEXT,summary_zh_cn TEXT DEFAULT '',summary_en_us TEXT DEFAULT '',created_at TIMESTAMPTZ DEFAULT NOW(),updated_at TIMESTAMPTZ DEFAULT NOW(),deleted_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_catalog_product_version(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT DEFAULT 0,product_no TEXT,version_no TEXT,version_number INT DEFAULT 1,status_code TEXT DEFAULT 'draft',source_code TEXT DEFAULT 'manual',published_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_catalog_billing_rule(tenant_id INT,version_no TEXT,status_code TEXT);
CREATE TABLE plugin_idc_core_catalog_price(tenant_id INT,version_no TEXT,status_code TEXT,currency_code TEXT);
INSERT INTO plugin_idc_core_catalog_category(category_no) VALUES('ctg_11111111111111111111111111111111');`
