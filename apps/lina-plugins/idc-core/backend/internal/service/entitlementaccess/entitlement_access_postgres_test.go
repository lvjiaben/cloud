// This file verifies the core-owned current-customer admission and rollback fence.
package entitlementaccess

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
)

type customerReader struct{ customer string }

func (r *customerReader) CurrentCustomer(context.Context) (*customerauthcap.Subject, error) {
	return &customerauthcap.Subject{CustomerNo: r.customer}, nil
}

func TestCurrentEntitlementAdmissionPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "entitlement_access_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	_, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no TEXT PRIMARY KEY,customer_no TEXT,order_no TEXT,status_code TEXT,source_intent_no TEXT,period_unit_code TEXT,period_count INT,started_at TIMESTAMPTZ,expires_at TIMESTAMPTZ,version BIGINT,entitlement_owner_id TEXT);
 CREATE TABLE plugin_idc_core_product_fulfillment_intent(tenant_id INT,intent_no TEXT,product_name_zh_cn TEXT,product_name_en_us TEXT);
 CREATE TABLE plugin_idc_core_refund_entitlement_lock(tenant_id INT,entitlement_no TEXT,released_at TIMESTAMPTZ);
 CREATE TABLE resource_callback_marker(value TEXT);
 INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES(0,'intent','测试商品','Test product');
 INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'ent_11111111111111111111111111111111','customer-a','order-a','active','intent','month',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 month',3,'idc-resource');`)
	if err != nil {
		t.Fatal(err)
	}
	reader := &customerReader{customer: "customer-a"}
	service, err := New(database, reader)
	if err != nil {
		t.Fatal(err)
	}
	number := "ent_11111111111111111111111111111111"
	item, err := service.ReadCurrentEntitlement(ctx, number)
	if err != nil || !item.CanOperate || item.ProductNameZhCN != "测试商品" || item.Version != 3 {
		t.Fatalf("owned service=%+v err=%v", item, err)
	}
	reader.customer = "customer-b"
	if _, err = service.ReadCurrentEntitlement(ctx, number); err == nil {
		t.Fatal("foreign service exposed")
	}
	reader.customer = "customer-b"
	queuedCalls := 0
	for _, version := range []int64{0, 2, 4} {
		if gateErr := service.WithQueuedEntitlement(ctx, number, version, func(context.Context, *customerauthcap.Entitlement) error { queuedCalls++; return nil }); gateErr == nil || queuedCalls != 0 {
			t.Fatal("stale queued authority crossed owner gate")
		}
	}
	if gateErr := service.WithQueuedEntitlement(ctx, number, 3, func(_ context.Context, e *customerauthcap.Entitlement) error {
		queuedCalls++
		if e.CustomerNo != "customer-a" {
			t.Fatal("incorrect queued owner")
		}
		return nil
	}); gateErr != nil || queuedCalls != 1 {
		t.Fatalf("queued authority admission: %v", gateErr)
	}
	reader.customer = "customer-a"
	forced := errors.New("rollback callback")
	err = service.WithCurrentEntitlement(ctx, number, func(txCtx context.Context, _ *customerauthcap.Entitlement) error {
		if _, err := database.Exec(txCtx, `INSERT INTO resource_callback_marker VALUES('must rollback')`); err != nil {
			return err
		}
		return forced
	})
	count, readErr := database.GetValue(ctx, `SELECT COUNT(*) FROM resource_callback_marker`)
	if !errors.Is(err, forced) || readErr != nil || count.Int() != 0 {
		t.Fatal("callback did not participate in entitlement transaction")
	}
	for _, test := range []struct{ name, sql string }{
		{"expired", `UPDATE plugin_idc_core_service_entitlement SET expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second'`},
		{"suspended", `UPDATE plugin_idc_core_service_entitlement SET expires_at=CURRENT_TIMESTAMP+INTERVAL '1 month',status_code='suspended'`},
		{"refund", `UPDATE plugin_idc_core_service_entitlement SET status_code='active';INSERT INTO plugin_idc_core_refund_entitlement_lock VALUES(0,'ent_11111111111111111111111111111111',NULL)`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err = database.Exec(ctx, test.sql); err != nil {
				t.Fatal(err)
			}
			called := false
			err = service.WithCurrentEntitlement(ctx, number, func(context.Context, *customerauthcap.Entitlement) error { called = true; return nil })
			if err == nil || called {
				t.Fatal("ineligible entitlement crossed write gate")
			}
			if gateErr := service.WithQueuedEntitlement(ctx, number, 3, func(context.Context, *customerauthcap.Entitlement) error { called = true; return nil }); gateErr == nil || called {
				t.Fatal("ineligible queued entitlement crossed write gate")
			}
		})
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_refund_entitlement_lock SET released_at=CURRENT_TIMESTAMP`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	locked := make(chan struct{})
	release := make(chan struct{})
	go func() {
		done <- service.WithCurrentEntitlement(ctx, number, func(context.Context, *customerauthcap.Entitlement) error { close(locked); <-release; return nil })
	}()
	<-locked
	competing := make(chan error, 1)
	go func() {
		_, e := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='suspended' WHERE entitlement_no=?`, number)
		competing <- e
	}()
	select {
	case <-competing:
		t.Fatal("entitlement write escaped held admission lock")
	case <-time.After(80 * time.Millisecond):
	}
	close(release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if err = <-competing; err != nil {
		t.Fatal(err)
	}
}
