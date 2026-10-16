// This file exercises directory migrations, optimistic writes and customer
// isolation in a disposable schema using local data only.
package productdirectory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type directoryAccessFake struct {
	customer string
	deny     bool
}

func (a *directoryAccessFake) Admin(context.Context, string) (int, error) {
	if a.deny {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return 7, nil
}
func (a *directoryAccessFake) Customer(context.Context) (string, error) {
	if a.deny {
		return "", bizerr.NewCode(CodeNotAccessible)
	}
	return a.customer, nil
}

type directoryAuditFake struct {
	fail  bool
	calls int
}

func (a *directoryAuditFake) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	if input.BeforeSnapshot == nil || input.AfterSnapshot == nil {
		return errors.New("missing audit snapshots")
	}
	if a.fail {
		return errors.New("audit persistence unavailable")
	}
	a.calls++
	return nil
}
func (a *directoryAuditFake) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}
func directoryDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN for directory PostgreSQL tests")
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_directory_" + hex.EncodeToString(nonce[:])
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, e := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); e != nil {
			t.Error(e)
		}
		if e := admin.Close(ctx); e != nil {
			t.Error(e)
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
		if e := db.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	// Minimal owner tables retain the production key and projection shapes. The
	// actual new migration is applied twice, including foreign keys and indexes.
	_, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_customer_subject(customer_no VARCHAR(40) PRIMARY KEY);CREATE TABLE plugin_idc_core_catalog_product(tenant_id INT DEFAULT 0,product_no VARCHAR(40) PRIMARY KEY,name_zh_cn VARCHAR(256),name_en_us VARCHAR(256),deleted_at TIMESTAMPTZ);CREATE TABLE plugin_idc_core_product_fulfillment_intent(tenant_id INT DEFAULT 0,intent_no VARCHAR(40) PRIMARY KEY,product_name_zh_cn VARCHAR(256),product_name_en_us VARCHAR(256));CREATE TABLE plugin_idc_core_service_entitlement(id BIGSERIAL PRIMARY KEY,tenant_id INT DEFAULT 0,entitlement_no VARCHAR(40),customer_no VARCHAR(40),product_no VARCHAR(40),source_intent_no VARCHAR(40),plugin_id VARCHAR(64),status_code VARCHAR(24),order_no VARCHAR(40),started_at TIMESTAMPTZ,expires_at TIMESTAMPTZ);INSERT INTO plugin_idc_core_customer_subject VALUES('customer-a'),('customer-b');INSERT INTO plugin_idc_core_catalog_product(product_no,name_zh_cn,name_en_us) VALUES('product-a','云服务器','Cloud server'),('product-b','SSL证书','SSL certificate'),('product-c','域名','Domain');INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,product_no,source_intent_no,plugin_id,status_code,order_no,started_at) VALUES('ent-a','customer-a','product-a','intent-a','idc-server','active','order-a',CURRENT_TIMESTAMP),('ent-b','customer-b','product-a','intent-b','idc-server','active','order-b',CURRENT_TIMESTAMP),('ent-c','customer-a','product-c','intent-c','domain','active','order-c',CURRENT_TIMESTAMP);INSERT INTO plugin_idc_core_product_fulfillment_intent(intent_no,product_name_zh_cn,product_name_en_us) VALUES('intent-a','冻结云服务器','Frozen cloud server')`)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "111-product-directory-navigation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		if _, err = db.Exec(ctx, string(source)); err != nil {
			t.Fatal(err)
		}
	}
	source, err = os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "129-member-navigation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		if _, err = db.Exec(ctx, string(source)); err != nil {
			t.Fatal(err)
		}
	}
	return db
}
func newDirectoryFixture(t *testing.T) (gdb.DB, Service, *directoryAccessFake, *directoryAuditFake) {
	t.Helper()
	db := directoryDatabase(t)
	if _, err := db.Exec(context.Background(), "DELETE FROM plugin_idc_core_product_directory WHERE navigation_type='system'"); err != nil {
		t.Fatal(err)
	}
	access := &directoryAccessFake{customer: "customer-a"}
	writer := &directoryAuditFake{}
	service, err := New(db, access, writer)
	if err != nil {
		t.Fatal(err)
	}
	return db, service, access, writer
}
func TestDirectoryMerchantMenusAndAllFallbackPostgreSQL(t *testing.T) {
	db, service, access, writer := newDirectoryFixture(t)
	ctx := context.Background()
	created, err := service.Save(ctx, SaveInput{NameZhCn: "云服务", NameEnUs: "Cloud services"})
	if err != nil {
		t.Fatal(err)
	}
	if created.DirectoryNo == AllDirectory || created.ProductCount != 0 || writer.calls != 1 {
		t.Fatalf("new navigation must not assign products: %#v", created)
	}
	if _, err := db.Exec(ctx, "INSERT INTO plugin_idc_core_product_directory_membership(product_no,directory_no) VALUES('product-a',?)", created.DirectoryNo); err != nil {
		t.Fatal(err)
	}
	dirs, err := service.ListAccount(ctx, 1, 100)
	if err != nil || len(dirs.Items) != 1 || dirs.Items[0].ServiceCount != 1 {
		t.Fatalf("default/customer-isolated directory: %#v %v", dirs, err)
	}
	// Historical preference records are retained but no longer affect navigation.
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_product_directory_visibility(customer_no,directory_no,hidden) VALUES('customer-a',?,TRUE)`, created.DirectoryNo); err != nil {
		t.Fatal(err)
	}
	dirs, err = service.ListAccount(ctx, 1, 100)
	if err != nil || dirs.Total != 1 || len(dirs.Items) != 1 || dirs.Items[0].DirectoryNo != created.DirectoryNo {
		t.Fatalf("obsolete preference hid merchant menu: %#v %v", dirs, err)
	}
	services, err := service.ListServices(ctx, created.DirectoryNo, 1, 20)
	if err != nil || services.Total != 1 || len(services.Items) != 1 || services.Items[0].EntitlementNo != "ent-a" || services.Items[0].OrderNo != "order-a" || services.Items[0].ProductNameZhCn != "冻结云服务器" || services.Items[0].StartedAt < 1000000000000 {
		t.Fatalf("merchant menu lost associated services or leaked customer: %#v %v", services, err)
	}
	access.customer = "customer-b"
	dirs, err = service.ListAccount(ctx, 1, 100)
	if err != nil || dirs.Items[0].ServiceCount != 1 {
		t.Fatal("merchant menu customer count leaked", err)
	}
	services, err = service.ListServices(ctx, AllDirectory, 1, 20)
	if err != nil || services.Total != 1 || services.Items[0].EntitlementNo != "ent-b" {
		t.Fatalf("all fallback crossed customer boundary: %#v %v", services, err)
	}
	access.customer = "customer-a"
	disabled, err := service.Save(ctx, SaveInput{DirectoryNo: created.DirectoryNo, NameZhCn: created.NameZhCn, NameEnUs: created.NameEnUs, StatusCode: StatusDisabled, ExpectedVersion: created.Version})
	if err != nil {
		t.Fatal(err)
	}
	dirs, err = service.ListAccount(ctx, 1, 100)
	if err != nil || dirs.Total != 0 {
		t.Fatal("disabled navigation remained visible", err)
	}
	if _, err = service.ListServices(ctx, created.DirectoryNo, 1, 20); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatal("disabled directory remained navigable", err)
	}
	services, err = service.ListServices(ctx, AllDirectory, 1, 1)
	if err != nil || services.Total != 2 || len(services.Items) != 1 {
		t.Fatal("all lost disabled or unclassified services", err)
	}
	next, err := service.ListServices(ctx, AllDirectory, 2, 1)
	if err != nil || len(next.Items) != 1 || next.Items[0].EntitlementNo == services.Items[0].EntitlementNo {
		t.Fatal("pagination repeated or lost service", err)
	}
	if _, err = service.Save(ctx, SaveInput{DirectoryNo: created.DirectoryNo, NameZhCn: created.NameZhCn, NameEnUs: created.NameEnUs, StatusCode: StatusEnabled, ExpectedVersion: disabled.Version}); err != nil {
		t.Fatal(err)
	}
	dirs, err = service.ListAccount(ctx, 1, 100)
	if err != nil || dirs.Total != 1 || len(dirs.Items) != 1 {
		t.Fatal("re-enable did not restore merchant menu", err)
	}
	value, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_service_entitlement`)
	if err != nil || value.Int() != 3 {
		t.Fatal("navigation changed service facts", err)
	}
}
func TestDirectoryMetadataPreservesAssignmentsPostgreSQL(t *testing.T) {
	db, service, access, writer := newDirectoryFixture(t)
	ctx := context.Background()
	first, err := service.Save(ctx, SaveInput{NameZhCn: "服务器", NameEnUs: "Servers"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO plugin_idc_core_product_directory_membership(product_no,directory_no) VALUES('product-a',?)", first.DirectoryNo); err != nil {
		t.Fatal(err)
	}
	update := SaveInput{DirectoryNo: first.DirectoryNo, NameZhCn: "改名后的菜单", NameEnUs: first.NameEnUs, StatusCode: StatusEnabled, ExpectedVersion: first.Version}
	changed, err := service.Save(ctx, update)
	if err != nil || changed.ProductCount != 1 {
		t.Fatalf("metadata changed assignment: %#v %v", changed, err)
	}
	if _, err = service.Save(ctx, update); !bizerr.Is(err, CodeConflict) {
		t.Fatal("stale version accepted", err)
	}
	writer.fail = true
	update.ExpectedVersion = changed.Version
	update.NameZhCn = "不应保存"
	if _, err = service.Save(ctx, update); err == nil {
		t.Fatal("audit failure accepted")
	}
	listed, err := service.ListAdmin(ctx, 1, 100)
	if err != nil || listed.Items[0].Version != changed.Version || listed.Items[0].NameZhCn != changed.NameZhCn || listed.Items[0].ProductCount != 1 {
		t.Fatal("audit failure modified metadata or assignment", err)
	}
	writer.fail = false
	update.NavigationType = NavigationCustom
	update.TargetURL = "https://example.com"
	if _, err = service.Save(ctx, update); !bizerr.Is(err, CodeHasProducts) {
		t.Fatal("linked product menu changed type", err)
	}
	listed, err = service.ListAdmin(ctx, 1, 100)
	if err != nil || listed.Items[0].NavigationType != NavigationProduct || listed.Items[0].ProductCount != 1 {
		t.Fatal("rejected type change modified mapping", err)
	}
	assignment, err := db.Model(do.ProductDirectoryMembership{}).Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: 0, ProductNo: "product-a"}).Value("directory_no")
	if err != nil || assignment.String() != first.DirectoryNo {
		t.Fatal("metadata operation changed stored product assignment", err)
	}
	access.deny = true
	if _, err = service.ListAdmin(ctx, 1, 100); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatal("unauthorized admin accepted", err)
	}
	if _, err = service.ListAccount(ctx, 1, 100); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatal("unauthorized customer accepted", err)
	}
}
func TestDirectoryPagingIncludesAllMerchantMenusPostgreSQL(t *testing.T) {
	db, service, _, _ := newDirectoryFixture(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_product_directory(directory_no,name_zh_cn,name_en_us,sort_order) SELECT 'directory_'||i,'目录'||i,'Directory '||i,i FROM generate_series(1,123) i`); err != nil {
		t.Fatal(err)
	}
	first, err := service.ListAccount(ctx, 1, 100)
	if err != nil || first.Total != 123 || len(first.Items) != 100 {
		t.Fatal("first menu page is incomplete", err)
	}
	second, err := service.ListAccount(ctx, 2, 100)
	if err != nil || second.Total != 123 || len(second.Items) != 23 {
		t.Fatal("second menu page is incomplete", err)
	}
	if first.Items[99].DirectoryNo == second.Items[0].DirectoryNo {
		t.Fatal("menu pagination repeated a directory")
	}
	for _, bounds := range [][2]int{{0, 20}, {1, 0}, {1, 101}, {1000001, 20}} {
		if _, err = service.ListAccount(ctx, bounds[0], bounds[1]); !bizerr.Is(err, CodeInvalid) {
			t.Fatal("unbounded pagination accepted", err)
		}
	}
	if _, err = service.ListServices(ctx, "directory_' OR TRUE --", 1, 20); !bizerr.Is(err, CodeInvalid) {
		t.Fatal("unsafe directory reference accepted", err)
	}
}

// TestDirectoryORMMaintainsUpdateTimesPostgreSQL verifies owner ORM writes,
// advance update time without application timestamps.
func TestDirectoryORMMaintainsUpdateTimesPostgreSQL(t *testing.T) {
	ctx := context.Background()
	db, service, _, _ := newDirectoryFixture(t)
	created, err := service.Save(ctx, SaveInput{NameZhCn: "时间目录", NameEnUs: "Timestamp directory"})
	if err != nil {
		t.Fatal(err)
	}
	// Fixture aging avoids a sleep and timestamp precision dependence. Production
	// writes must replace this old value while preserving the original created_at.
	_, err = db.Exec(ctx, `UPDATE plugin_idc_core_product_directory SET updated_at='2000-01-01T00:00:00Z' WHERE directory_no=?`, created.DirectoryNo)
	if err != nil {
		t.Fatal(err)
	}
	before, err := db.GetValue(ctx, `SELECT created_at FROM plugin_idc_core_product_directory WHERE directory_no=?`, created.DirectoryNo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Save(ctx, SaveInput{DirectoryNo: created.DirectoryNo, NameZhCn: created.NameZhCn, NameEnUs: created.NameEnUs, StatusCode: StatusEnabled, ExpectedVersion: created.Version})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := db.GetValue(ctx, `SELECT updated_at>'2000-01-01T00:00:00Z'::timestamptz AND created_at=?::timestamptz FROM plugin_idc_core_product_directory WHERE directory_no=?`, before.Time(), created.DirectoryNo)
	if err != nil || !changed.Bool() {
		t.Fatalf("ORM directory time maintenance: value=%v err=%v", changed, err)
	}
}

// A menu may contain more products than the old navigation editor's 200-item
// selection limit; listing and metadata writes return counts, never full rows.
func TestDirectoryProductCountBeyondLegacySelectionPostgreSQL(t *testing.T) {
	db, service, _, _ := newDirectoryFixture(t)
	ctx := context.Background()
	menu, err := service.Save(ctx, SaveInput{NameZhCn: "大目录", NameEnUs: "Large menu"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product(product_no,name_zh_cn,name_en_us) SELECT 'bulk-'||i,'批量商品','Bulk product' FROM generate_series(1,201) AS i`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_product_directory_membership(product_no,directory_no) SELECT product_no,? FROM plugin_idc_core_catalog_product WHERE product_no LIKE 'bulk-%'`, menu.DirectoryNo); err != nil {
		t.Fatal(err)
	}
	listed, err := service.ListAdmin(ctx, 1, 20)
	if err != nil || len(listed.Items) != 1 || listed.Items[0].ProductCount != 201 {
		t.Fatalf("aggregate omitted products: %#v %v", listed, err)
	}
	changed, err := service.Save(ctx, SaveInput{DirectoryNo: menu.DirectoryNo, NameZhCn: "改名", NameEnUs: "Renamed", StatusCode: StatusEnabled, ExpectedVersion: menu.Version})
	if err != nil || changed.ProductCount != 201 {
		t.Fatalf("metadata lost aggregate: %#v %v", changed, err)
	}
	count, err := db.Model(do.ProductDirectoryMembership{}).Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: 0, DirectoryNo: menu.DirectoryNo}).Count()
	if err != nil || count != 201 {
		t.Fatal("metadata changed stored membership", count, err)
	}
}
