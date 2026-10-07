// This file verifies missing-cost pagination against isolated PostgreSQL read
// projections, including consumed campaign bindings and owner scope.

package marketing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestMissingCostNumberedPagesPostgreSQL(t *testing.T) {
	ctx := context.Background()
	db := missingCostNumberedDatabase(t, ctx)
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_commerce_order (
 order_no TEXT PRIMARY KEY, quote_no TEXT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_item (
 order_no TEXT NOT NULL, order_item_no TEXT PRIMARY KEY, product_name_zh_cn TEXT NOT NULL, product_name_en_us TEXT NOT NULL
);
CREATE TABLE plugin_idc_core_marketing_campaign_reservation (
 quote_reference TEXT NOT NULL, status_code TEXT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_cost_snapshot (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 order_item_no TEXT NOT NULL UNIQUE, order_no TEXT NOT NULL, version_no TEXT NOT NULL DEFAULT 'version', currency_code TEXT NOT NULL DEFAULT 'CNY',
 period_unit_code TEXT NOT NULL DEFAULT 'month', period_count INT NOT NULL DEFAULT 1, quantity INT NOT NULL DEFAULT 1,
 purchase_cycles INT NOT NULL DEFAULT 1, status_code TEXT NOT NULL, unit_cost_minor BIGINT, total_cost_minor BIGINT,
 correction_reason TEXT, version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO plugin_idc_core_commerce_order SELECT 'order-page-'||n,'quote-page-'||n FROM generate_series(1,30) n;
INSERT INTO plugin_idc_core_commerce_order_item SELECT 'order-page-'||n,'item-page-'||n,'分页商品'||n,'Product '||n FROM generate_series(1,30) n;
INSERT INTO plugin_idc_core_marketing_campaign_reservation SELECT 'quote-page-'||n,'consumed' FROM generate_series(1,28) n;
INSERT INTO plugin_idc_core_marketing_campaign_reservation VALUES ('quote-page-1','consumed'),('quote-page-29','reserved');
INSERT INTO plugin_idc_core_commerce_order_cost_snapshot (tenant_id,order_item_no,order_no,status_code)
 SELECT CASE WHEN n=28 THEN 9 ELSE 0 END,'item-page-'||n,'order-page-'||n,CASE WHEN n IN (26,27) THEN 'frozen' ELSE 'missing' END FROM generate_series(1,30) n;`); err != nil {
		t.Fatal(err)
	}
	service, err := NewMarketingInsightsService(db, &marketingInsightsAuthorizerStub{}, &marketingActionAuditIntegrationStub{})
	if err != nil {
		t.Fatal(err)
	}
	assertMarketingNumberedPages(t, 25, func(page, size int) ([]string, int, error) {
		result, readErr := service.MissingCosts(ctx, MarketingMissingCostSearchInput{Page: page, Limit: size})
		if readErr != nil {
			return nil, 0, readErr
		}
		ids := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			ids = append(ids, item.OrderItemNo)
		}
		return ids, result.Total, nil
	})
	denied := &marketingInsightsServiceImpl{database: db, authorizer: &numberedInsightsDenied{}}
	if _, err := denied.MissingCosts(ctx, MarketingMissingCostSearchInput{Page: 1, Limit: 10}); err != context.Canceled {
		t.Fatalf("authorization ignored: %v", err)
	}
}

func missingCostNumberedDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN for missing-cost pagination PostgreSQL test")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_missing_cost_page_" + hex.EncodeToString(nonce[:])
	quote := `"` + schema + `"`
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quote); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quote+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return db
}
