// This file tests actual catalog import writes, actor-scoped request replay,
// concurrent submissions and total rollback in an isolated PostgreSQL schema.

package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func TestCatalogImportAtomicPersistencePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, catalogImportMigrationFixtureSQL+catalogImportPersistenceFixtureSQL); err != nil {
		t.Fatal(err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "110-catalog-product-import.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	service, err := newManagementService(&managementTestAudit{}, &managementTestAuthorizer{actor: 7}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	input := ImportInput{RequestKey: "request-import-000001", InterfaceInstanceNo: "pfi_" + strings.Repeat("1", 32), Reference: "101", CategoryNo: "ctg_" + strings.Repeat("1", 32), PricingMode: ImportPricingMarkup, MarkupPercent: "10"}
	source := &resourceinterfaceprojectioncap.CatalogDetail{Product: resourceinterfaceprojectioncap.CatalogProduct{Reference: "101", Name: "原始上游名称", Description: "Supplier description"}, PriceScheduleComplete: true,
		BasePrices: []resourceinterfaceprojectioncap.CatalogPrice{{CurrencyCode: "CNY", Period: resourceinterfaceprojectioncap.CatalogPeriodMonth, PeriodCount: 1, Amount: "100", SetupFee: "0"}, {CurrencyCode: "USD", Period: resourceinterfaceprojectioncap.CatalogPeriodMonth, PeriodCount: 1, Amount: "0.005", SetupFee: "0"}}}
	missing := []ImportMissingConfiguration{{FieldKey: "cpu", NameZhCN: "处理器", NameEnUS: "CPU"}}
	var result *ImportResult
	commit := func(input ImportInput, fail bool) error {
		return database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
			var commitErr error
			result, _, commitErr = service.CommitImport(txCtx, input, source, missing)
			if commitErr != nil {
				return commitErr
			}
			if fail {
				return errors.New("binding failed")
			}
			return nil
		})
	}
	if err = commit(input, true); err == nil {
		t.Fatal("rollback fixture succeeded")
	}
	count, err := database.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product_import`)
	if err != nil || count != 0 {
		t.Fatalf("rollback retained request: %d %v", count, err)
	}
	count, err = database.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product WHERE public_slug LIKE 'import-%'`)
	if err != nil || count != 0 {
		t.Fatalf("rollback retained product: %d %v", count, err)
	}
	if err = commit(input, false); err != nil {
		t.Fatal(err)
	}
	if result.Item.Status != catalogStatusDraft || result.Item.VersionStatus != catalogStatusDraft || result.PricingIncomplete {
		t.Fatalf("bad draft result: %+v", result)
	}
	productNo := result.Item.ProductNo
	row, err := database.GetOne(ctx, `SELECT p.name_zh_cn,p.name_en_us,p.current_version_no,v.contract_policy_code,i.tracking_enabled,i.sellable_quantity
FROM plugin_idc_core_catalog_product p JOIN plugin_idc_core_catalog_product_version v USING(product_no)
JOIN plugin_idc_core_catalog_product_version_inventory i USING(version_no) WHERE p.product_no=?`, productNo)
	if err != nil || row["name_zh_cn"].String() != source.Product.Name || row["name_en_us"].String() != source.Product.Name || !row["current_version_no"].IsNil() || row["contract_policy_code"].String() != "inherit" || row["tracking_enabled"].Bool() || row["sellable_quantity"].Int64() != 0 {
		t.Fatalf("ordinary draft defaults changed: %+v %v", row, err)
	}
	prices, err := database.GetAll(ctx, `SELECT currency_code,amount_minor,renewal_amount_minor FROM plugin_idc_core_catalog_price ORDER BY currency_code`)
	if err != nil || len(prices) != 2 || prices[0]["amount_minor"].Int64() != 11000 || prices[1]["amount_minor"].Int64() != 1 || !prices[0]["renewal_amount_minor"].IsNil() {
		t.Fatalf("exact import prices: %+v %v", prices, err)
	}
	replay, err := service.LookupImport(ctx, input)
	if err != nil || replay == nil || replay.Item.ProductNo != productNo {
		t.Fatalf("local replay=%+v err=%v", replay, err)
	}
	changed := input
	changed.MarkupPercent = "11"
	if _, err = service.LookupImport(ctx, changed); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("changed same-key request accepted: %v", err)
	}
	// Competing transactions must converge on the same newly created product.
	input.RequestKey = "request-import-concurrent"
	var wait sync.WaitGroup
	results := make(chan *ImportResult, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			var imported *ImportResult
			writeErr := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
				var err error
				imported, _, err = service.CommitImport(txCtx, input, source, missing)
				return err
			})
			if writeErr != nil {
				t.Errorf("concurrent import: %v", writeErr)
				return
			}
			results <- imported
		}()
	}
	wait.Wait()
	close(results)
	identity := ""
	for imported := range results {
		if identity == "" {
			identity = imported.Item.ProductNo
		}
		if imported.Item.ProductNo != identity {
			t.Fatal("concurrent request duplicated product")
		}
	}
	count, err = database.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product_import`)
	if err != nil || count != 2 {
		t.Fatalf("two explicit requests should create exactly two products: %d %v", count, err)
	}
}

const catalogImportPersistenceFixtureSQL = `
ALTER TABLE plugin_idc_core_catalog_product ADD COLUMN category_no TEXT,ADD COLUMN public_slug TEXT UNIQUE,ADD COLUMN name_zh_cn TEXT,ADD COLUMN name_en_us TEXT,
ADD COLUMN summary_zh_cn TEXT,ADD COLUMN summary_en_us TEXT,ADD COLUMN current_version_no TEXT,ADD COLUMN sort_order INT,ADD COLUMN created_by INT,ADD COLUMN updated_by INT,
ADD COLUMN created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,ADD COLUMN updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE plugin_idc_core_catalog_product_version ADD COLUMN version_number INT,ADD COLUMN source_code TEXT,ADD COLUMN requires_contract BOOL,
ADD COLUMN contract_policy_code TEXT,ADD COLUMN contract_template_no TEXT,ADD COLUMN contract_policy_version BIGINT,ADD COLUMN created_by INT,ADD COLUMN created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
CREATE TABLE plugin_idc_core_catalog_category(tenant_id INT,category_no TEXT,status_code TEXT,name_zh_cn TEXT,name_en_us TEXT);
INSERT INTO plugin_idc_core_catalog_category VALUES(0,'ctg_11111111111111111111111111111111','active','分类','Category');
CREATE TABLE plugin_idc_core_catalog_product_version_inventory(version_no TEXT PRIMARY KEY,tracking_enabled BOOL,sellable_quantity BIGINT,version BIGINT,updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_catalog_billing_rule(tenant_id INT,billing_rule_no TEXT PRIMARY KEY,version_no TEXT,rule_code TEXT,charge_type_code TEXT,period_unit_code TEXT,
period_count INT,price_phase_code TEXT,quantity_mode_code TEXT,trial_quantity INT,status_code TEXT,minimum_purchase_cycles INT,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_catalog_price(tenant_id INT,price_no TEXT PRIMARY KEY,version_no TEXT,billing_rule_no TEXT,currency_code TEXT,amount_minor BIGINT,
setup_fee_minor BIGINT,renewal_amount_minor BIGINT,minimum_quantity BIGINT,status_code TEXT,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);
`
