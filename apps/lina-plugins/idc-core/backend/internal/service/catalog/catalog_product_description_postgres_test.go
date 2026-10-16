// This file verifies optimistic metadata writes, isolation and audit rollback
// using a disposable PostgreSQL schema without modifying installed products.

package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type descriptionAudit struct {
	idccap.BusinessAuditWriter
	fail  bool
	calls int
}

func (a *descriptionAudit) AppendBusinessTransition(_ context.Context, event idccap.BusinessAuditTransition) error {
	a.calls++
	before, beforeOK := event.BeforeSnapshot.(map[string]any)
	after, afterOK := event.AfterSnapshot.(map[string]any)
	if !beforeOK || !afterOK || event.RequiredPermission != permissionCatalogProductUpdate || before["summary_zh_cn"] != "" && before["summary_zh_cn"] != "介绍" || after["summary_zh_cn"] != "介绍" {
		return errors.New("incorrect audit")
	}
	if a.fail {
		return errors.New("audit unavailable")
	}
	return nil
}

func TestProductDescriptionUpdatePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product(id BIGSERIAL,tenant_id INT,product_no TEXT,name_zh_cn TEXT,name_en_us TEXT,summary_zh_cn TEXT,summary_en_us TEXT,public_slug TEXT,version BIGINT,updated_by INT,updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ); INSERT INTO plugin_idc_core_catalog_product VALUES(DEFAULT,0,'prd_11111111111111111111111111111111','商品','Product','','','unchanged',1,7,NOW(),NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_product_directory(directory_no TEXT PRIMARY KEY,tenant_id INT,status_code TEXT,navigation_type TEXT DEFAULT 'product'); CREATE TABLE plugin_idc_core_product_directory_membership(product_no TEXT PRIMARY KEY,tenant_id INT,directory_no TEXT); INSERT INTO plugin_idc_core_product_directory VALUES('pdr_one',0,'enabled','product'),('pdr_two',0,'enabled','product'),('pdr_disabled',0,'disabled','product')`); err != nil {
		t.Fatal(err)
	}
	audit := &descriptionAudit{fail: true}
	auth := &managementTestAuthorizer{actor: 7}
	svc, err := newManagementService(audit, auth, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directoryNo := "pdr_one"
	input := ProductDescriptionUpdateInput{DirectoryNo: &directoryNo, ProductNo: "prd_" + strings.Repeat("1", 32), Version: 1, NameZhCN: "名称", NameEnUS: "Name", SummaryZhCN: "介绍", SummaryEnUS: "Summary"}
	if _, err = svc.UpdateProductDescription(ctx, input); err == nil {
		t.Fatal("audit failure committed")
	}
	version, err := db.GetValue(ctx, `SELECT version FROM plugin_idc_core_catalog_product`)
	if err != nil || version.Int64() != 1 {
		t.Fatal("rollback lost")
	}
	count, e := db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_product_directory_membership")
	if e != nil || count.Int() != 0 {
		t.Fatal("audit rollback kept menu assignment")
	}
	audit.fail = false
	if got, e := svc.UpdateProductDescription(ctx, input); e != nil || got != 2 {
		t.Fatalf("update: %d %v", got, e)
	}
	if _, err = svc.UpdateProductDescription(ctx, input); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("stale write: %v", err)
	}
	slug, err := db.GetValue(ctx, `SELECT public_slug FROM plugin_idc_core_catalog_product`)
	if err != nil || slug.String() != "unchanged" {
		t.Fatal("URL changed")
	}
	menu, err := db.GetValue(ctx, "SELECT directory_no FROM plugin_idc_core_product_directory_membership")
	if err != nil || menu.String() != "pdr_one" {
		t.Fatal("saved menu missing")
	}
	input.Version = 2
	directoryNo = "pdr_two"
	if got, e := svc.UpdateProductDescription(ctx, input); e != nil || got != 3 {
		t.Fatalf("replace menu: %d %v", got, e)
	}
	menu, err = db.GetValue(ctx, "SELECT directory_no FROM plugin_idc_core_product_directory_membership")
	if err != nil || menu.String() != "pdr_two" {
		t.Fatal("menu replacement missing")
	}
	input.Version = 3
	directoryNo = "pdr_disabled"
	if _, e := svc.UpdateProductDescription(ctx, input); !bizerr.Is(e, CodeManagementNotAccessible) {
		t.Fatalf("disabled menu accepted: %v", e)
	}
	directoryNo = ""
	if got, e := svc.UpdateProductDescription(ctx, input); e != nil || got != 4 {
		t.Fatalf("clear menu: %d %v", got, e)
	}
	count, e = db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_product_directory_membership")
	if e != nil || count.Int() != 0 {
		t.Fatal("menu clearing retained assignment")
	}
	input.Version = 4
	input.SummaryEnUS = ""
	if _, err = svc.UpdateProductDescription(ctx, input); !bizerr.Is(err, CodeManagementInvalid) {
		t.Fatal("empty copy accepted")
	}
	input.SummaryEnUS = "Summary"
	auth.actor = 0
	if _, err = svc.UpdateProductDescription(ctx, input); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatal("unauthorized write")
	}
	auth.actor = 7
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET tenant_id=9`); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateProductDescription(ctx, input); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatal("foreign tenant visible")
	}
	if audit.calls != 4 {
		t.Fatalf("invalid writes audited: %d", audit.calls)
	}
}

// A concurrent menu-type update and product assignment must serialize on the
// destination row. Without its row lock, the assignment would commit to a
// destination that is no longer a product menu.
func TestProductMenuAssignmentLocksDestinationPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, "CREATE TABLE plugin_idc_core_product_directory(directory_no TEXT PRIMARY KEY,tenant_id INT,status_code TEXT,navigation_type TEXT); CREATE TABLE plugin_idc_core_product_directory_membership(product_no TEXT PRIMARY KEY,tenant_id INT,directory_no TEXT); INSERT INTO plugin_idc_core_product_directory VALUES('pdr_target',0,'enabled','product')"); err != nil {
		t.Fatal(err)
	}
	menuTX, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = menuTX.Rollback() })
	if _, err = menuTX.GetValue("SELECT directory_no FROM plugin_idc_core_product_directory WHERE directory_no='pdr_target' FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	assignErr := db.Transaction(context.Background(), func(assignCtx context.Context, _ gdb.TX) error {
		if _, err := db.Exec(assignCtx, "SET LOCAL lock_timeout='150ms'"); err != nil {
			return err
		}
		return replaceProductDirectory(assignCtx, "prd_assignment", "pdr_target")
	})
	if assignErr == nil || !strings.Contains(assignErr.Error(), "lock timeout") {
		t.Fatalf("assignment ignored destination row lock: %v", assignErr)
	}
	if _, err = menuTX.Exec("UPDATE plugin_idc_core_product_directory SET navigation_type='system' WHERE directory_no='pdr_target'"); err != nil {
		t.Fatal(err)
	}
	if err = menuTX.Commit(); err != nil {
		t.Fatal(err)
	}
	err = db.Transaction(context.Background(), func(assignCtx context.Context, _ gdb.TX) error {
		return replaceProductDirectory(assignCtx, "prd_assignment", "pdr_target")
	})
	if !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("non-product destination accepted: %v", err)
	}
	count, err := db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_product_directory_membership")
	if err != nil || count.Int() != 0 {
		t.Fatal("rejected concurrent assignment changed membership", err)
	}
}
