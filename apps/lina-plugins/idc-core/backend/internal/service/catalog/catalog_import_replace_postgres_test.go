// This file proves existing-product upstream replacement and rollback using a
// disposable database schema. Earlier versions, copy and service facts survive.

package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExistingProductUpstreamReplacementPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, catalogImportMigrationFixtureSQL+catalogImportPersistenceFixtureSQL+portableImportFixtureSQL); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"catalog_import_candidate", "catalog_import_term", "catalog_import_contract", "catalog_product_version_configuration", "catalog_product_version_fulfillment_policy", "catalog_product_version_fulfillment_binding"} {
		if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_`+table+`(tenant_id INT,version_no TEXT)`); err != nil {
			t.Fatal(err)
		}
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source path")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/110-catalog-product-import.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	target := "ver_" + strings.Repeat("1", 32)
	product := "prd_" + strings.Repeat("1", 32)
	if _, err = db.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product ADD COLUMN version BIGINT DEFAULT 1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET category_no='ctg_11111111111111111111111111111111',name_zh_cn='保留名称' WHERE product_no=?`, product); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET version_number=2,source_code='manual' WHERE version_no=?`, target); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"catalog_product_version_configuration", "catalog_product_version_fulfillment_binding"} {
		if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_`+table+`(tenant_id,version_no) VALUES(0,?),(0,'historical')`, target); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_price(tenant_id,price_no,version_no) VALUES(0,'old-draft',?),(0,'old-history','historical')`, target); err != nil {
		t.Fatal(err)
	}
	svc, err := newManagementService(&managementTestAudit{}, &managementTestAuthorizer{actor: 7}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	input := ImportInput{TargetVersionNo: target, RequestKey: "existing-upstream-request-001", InterfaceInstanceNo: "pfi_" + strings.Repeat("1", 32), Reference: "101", CategoryNo: "ctg_" + strings.Repeat("1", 32), PricingMode: ImportPricingMarkup, MarkupPercent: "10"}
	source := &resourceinterfaceprojectioncap.CatalogDetail{Product: resourceinterfaceprojectioncap.CatalogProduct{Reference: "101", Name: "新的上游名称"}, PriceScheduleComplete: true, BasePrices: []resourceinterfaceprojectioncap.CatalogPrice{{CurrencyCode: "CNY", Period: resourceinterfaceprojectioncap.CatalogPeriodMonth, PeriodCount: 1, Amount: "100.00", SetupFee: "0"}}}
	var result *ImportResult
	commit := func(fail bool) error {
		return db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
			var writeErr error
			result, _, writeErr = svc.CommitImport(txCtx, input, source, nil)
			if writeErr != nil {
				return writeErr
			}
			if fail {
				return errors.New("later binding write failed")
			}
			return nil
		})
	}
	if err = commit(true); err == nil {
		t.Fatal("failed binding did not roll back")
	}
	for _, table := range []string{"catalog_price", "catalog_product_version_configuration", "catalog_product_version_fulfillment_binding"} {
		count, countErr := db.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_`+table+` WHERE version_no=?`, target)
		if countErr != nil || count != 1 {
			t.Fatalf("rollback lost %s: %d %v", table, count, countErr)
		}
	}
	if err = commit(false); err != nil {
		t.Fatal(err)
	}
	if result.Item.ProductNo != product || result.Item.VersionNo != target {
		t.Fatal("replacement created another product", result)
	}
	amount, err := db.GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE version_no=?`, target)
	if err != nil || amount.Int64() != 11000 {
		t.Fatal("incorrect markup", amount, err)
	}
	name, err := db.GetValue(ctx, `SELECT name_zh_cn FROM plugin_idc_core_catalog_product WHERE product_no=?`, product)
	if err != nil || name.String() != "保留名称" {
		t.Fatal("replacement changed sales copy", name, err)
	}
	for _, table := range []string{"catalog_price", "catalog_product_version_configuration", "catalog_product_version_fulfillment_binding"} {
		count, countErr := db.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_`+table+` WHERE version_no='historical'`)
		if countErr != nil || count != 1 {
			t.Fatalf("replacement changed history %s: %d %v", table, count, countErr)
		}
	}
	replay, replayErr := svc.LookupImport(ctx, input)
	if replayErr != nil || replay == nil || replay.Item.ProductNo != product {
		t.Fatal("unchanged retry lost identity", replay, replayErr)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_import_contract(tenant_id,version_no) VALUES(0,?)`, target); err != nil {
		t.Fatal(err)
	}
	locked := input
	locked.RequestKey = "locked-upstream-replace-001"
	err = db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		_, _, e := svc.CommitImport(txCtx, locked, source, nil)
		return e
	})
	if !bizerr.Is(err, CodeUpstreamConfigurationLocked) {
		t.Fatal("protected source was not rejected", err)
	}
	wrong := input
	wrong.RequestKey = "existing-upstream-other-01"
	wrong.TargetVersionNo = "ver_" + strings.Repeat("3", 32)
	err = db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		_, _, e := svc.CommitImport(txCtx, wrong, source, nil)
		return e
	})
	if !bizerr.Is(err, CodePublicationNotAccessible) {
		t.Fatal("other tenant was not rejected", err)
	}
}
