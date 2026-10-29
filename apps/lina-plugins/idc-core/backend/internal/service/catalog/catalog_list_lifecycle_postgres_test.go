// This file verifies page-bounded stock and delivery facts plus recoverable
// deletion in disposable PostgreSQL schemas; merchant data is never modified.
package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"os"
	"testing"
)

type deletionAudit struct {
	idccap.BusinessAuditWriter
	fail  bool
	calls int
}

func (a *deletionAudit) AppendBusinessTransition(_ context.Context, event idccap.BusinessAuditTransition) error {
	a.calls++
	if event.RequiredPermission != permissionCatalogProductDelete || event.OperationCode != "catalog.product.delete" {
		return errors.New("wrong deletion audit")
	}
	if a.fail {
		return errors.New("audit unavailable")
	}
	return nil
}

func TestCatalogManagementLifecycleStatisticsPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, catalogNumberedPageFixtureSQL+`
 INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,name_zh_cn,name_en_us,public_slug,sort_order) VALUES('prd_11111111111111111111111111111111','ctg_11111111111111111111111111111111','商品','Product','test',-12),('prd_22222222222222222222222222222222','ctg_11111111111111111111111111111111','不限','Unlimited','unlimited',20);
 INSERT INTO plugin_idc_core_catalog_product_version(product_no,version_no,version_number) VALUES('prd_11111111111111111111111111111111','ver_11111111111111111111111111111111',2),('prd_11111111111111111111111111111111','ver_33333333333333333333333333333333',1),('prd_22222222222222222222222222222222','ver_22222222222222222222222222222222',1);
 INSERT INTO plugin_idc_core_catalog_product_version_inventory VALUES('ver_11111111111111111111111111111111',TRUE,12),('ver_22222222222222222222222222222222',FALSE,0);
 INSERT INTO plugin_idc_core_commerce_inventory_reservation VALUES(0,'ver_11111111111111111111111111111111',3,'held'),(0,'ver_33333333333333333333333333333333',2,'committed'),(0,'ver_33333333333333333333333333333333',8,'released'),(9,'ver_11111111111111111111111111111111',9,'held');
 INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES(0,'a','prd_11111111111111111111111111111111','completed','per_quantity_unit',10),(0,'b','prd_11111111111111111111111111111111','completed','per_quantity_unit',10),(0,'c','prd_11111111111111111111111111111111','completed','per_order_item',3),(0,'d','prd_11111111111111111111111111111111','incomplete','per_quantity_unit',1),(9,'e','prd_11111111111111111111111111111111','completed','per_quantity_unit',1);
 INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'ent-a','a','active'),(0,'ent-b','b','terminated');`); err != nil {
		t.Fatal(err)
	}
	indexSQL, err := os.ReadFile("../../../../manifest/sql/146-catalog-list-delivery-statistics.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = db.Exec(ctx, string(indexSQL)); err != nil {
			t.Fatal("statistics index must be repeatable", err)
		}
	}
	input, err := normalizeProductManagementSearch(ProductManagementSearchInput{Page: 1, Limit: 10}, testProductManagementCursorCodec(t))
	if err != nil {
		t.Fatal(err)
	}
	page, err := searchProductManagementPage(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	first := page.Products[0]
	if first.SortOrder != -12 || first.AvailableQuantity == nil || *first.AvailableQuantity != 7 || first.ProvisionedQuantity != 4 || first.TotalQuantity != 5 {
		t.Fatalf("wrong lifecycle projection: %+v", first)
	}
	if page.Products[1].AvailableQuantity != nil || page.Products[1].TotalQuantity != 0 {
		t.Fatalf("unlimited/zero lost: %+v", page.Products[1])
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory SET sellable_quantity=1 WHERE tracking_enabled`); err != nil {
		t.Fatal(err)
	}
	page, err = searchProductManagementPage(ctx, input)
	if err != nil || page.Products[0].AvailableQuantity == nil || *page.Products[0].AvailableQuantity != 0 {
		t.Fatalf("stock must clamp at zero: %+v %v", page, err)
	}
}

func TestCatalogProductDeletionPreservesHistoryPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	const product = "prd_11111111111111111111111111111111"
	if _, err := db.Exec(ctx, catalogNumberedPageFixtureSQL+`INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,name_zh_cn,name_en_us,public_slug) VALUES('prd_11111111111111111111111111111111','ctg_11111111111111111111111111111111','商品','Product','test');
 INSERT INTO plugin_idc_core_catalog_product_version(product_no,version_no) VALUES('prd_11111111111111111111111111111111','ver_11111111111111111111111111111111');
 INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'existing-service','intent-existing','active');`); err != nil {
		t.Fatal(err)
	}
	audit := &deletionAudit{fail: true}
	auth := &managementTestAuthorizer{actor: 7}
	svc, err := newManagementService(audit, auth, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	input := ProductDeleteInput{ProductNo: product, Version: 1}
	if err = svc.DeleteProduct(ctx, input); err == nil {
		t.Fatal("audit failure should rollback")
	}
	count, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_product WHERE deleted_at IS NULL AND version=1`)
	if err != nil || count.Int() != 1 {
		t.Fatal("deletion escaped audit rollback", err)
	}
	audit.fail = false
	input.Version = 2
	if err = svc.DeleteProduct(ctx, input); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatal("stale deletion accepted", err)
	}
	input.Version = 1
	auth.err = bizerr.NewCode(CodeManagementNotAccessible)
	if err = svc.DeleteProduct(ctx, input); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatal("denied deletion accepted", err)
	}
	auth.err = nil
	if err = svc.DeleteProduct(ctx, input); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetOne(ctx, `SELECT status_code,version,deleted_at FROM plugin_idc_core_catalog_product WHERE product_no=?`, product)
	if err != nil || row["deleted_at"].IsNil() || row["version"].Int() != 2 || row["status_code"].String() != "retired" {
		t.Fatal("wrong deletion", row, err)
	}
	pageInput, _ := normalizeProductManagementSearch(ProductManagementSearchInput{Page: 1, Limit: 10}, testProductManagementCursorCodec(t))
	page, err := searchProductManagementPage(ctx, pageInput)
	if err != nil || page.Total != 0 {
		t.Fatal("deleted product remains listed", err)
	}
	reader := &publicCatalogReaderImpl{database: db}
	if err = reader.RequireRenewalSale(ctx, product); err != nil {
		t.Fatal("deleted product blocked existing renewal", err)
	}
	count, err = db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_service_entitlement WHERE entitlement_no='existing-service' AND status_code='active'`)
	if err != nil || count.Int() != 1 {
		t.Fatal("service modified", err)
	}
	if err = svc.DeleteProduct(ctx, input); err == nil {
		t.Fatal("duplicate delete unexpectedly succeeded")
	}
}
