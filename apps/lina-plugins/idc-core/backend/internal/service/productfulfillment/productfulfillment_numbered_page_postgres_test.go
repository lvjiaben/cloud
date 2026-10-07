package productfulfillment

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

type numberedIntentAuthorizer struct {
	ManagementAuthorizer
	allowed []string
	denied  bool
}

func (a *numberedIntentAuthorizer) AuthorizeList(context.Context) (Authorization, error) {
	if a.denied {
		return Authorization{}, context.Canceled
	}
	return Authorization{ActorUserID: 1, RequiredPermission: PermissionTaskList, DataScope: "test-customers", AllowedCustomerNos: a.allowed}, nil
}

// This read-contract fixture intentionally models only projected columns. It
// tests real PostgreSQL scope/count/order/offset, not installation migrations.
func TestIntentNumberedPagesPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_intent_pages_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteOutcomeIdentifier(schema)); err != nil {
		_ = admin.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(schema, "idc_intent_pages_") {
			t.Error("unexpected schema")
		} else if _, e := admin.Exec(ctx, "DROP SCHEMA "+quoteOutcomeIdentifier(schema)+" CASCADE"); e != nil {
			t.Error(e)
		}
		if e := admin.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendOutcomeSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := db.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	_, err = db.Exec(ctx, `
CREATE TABLE plugin_idc_core_product_fulfillment_intent (
 tenant_id int NOT NULL DEFAULT 0,intent_no text PRIMARY KEY,order_no text,order_item_no text,customer_no text,
 product_name_zh_cn text DEFAULT '分页产品',product_name_en_us text DEFAULT 'Paged product',strategy_code text DEFAULT 'manual',
 granularity_code text DEFAULT 'per_order_item',status_code text DEFAULT 'manual_pending',quantity int DEFAULT 1,unit_sequence int DEFAULT 1,
 version bigint DEFAULT 1,financially_settled_at timestamptz DEFAULT CURRENT_TIMESTAMP,updated_at timestamptz DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_product_fulfillment_task (tenant_id int,intent_no text,takeover_requested bool);
CREATE TABLE plugin_idc_core_customer_subject (tenant_id int DEFAULT 0,customer_no text,customer_code bigint,display_name text);
CREATE TABLE plugin_idc_core_customer_contact (id int,tenant_id int,customer_no text,contact_no text,status_code text,phone_country_code text,phone text);
CREATE TABLE plugin_idc_core_customer_contact_role (tenant_id int,customer_no text,contact_no text,role_code text,status_code text);
INSERT INTO plugin_idc_core_customer_subject (customer_no,customer_code,display_name) VALUES ('customer-a',10001,'Customer A'),('customer-b',10002,'Customer B');
INSERT INTO plugin_idc_core_product_fulfillment_intent (intent_no,order_no,order_item_no,customer_no)
 SELECT 'intent-'||lpad(n::text,3,'0'),'order-'||n,'item-'||n,'customer-a' FROM generate_series(1,25) n;
INSERT INTO plugin_idc_core_product_fulfillment_intent (intent_no,order_no,order_item_no,customer_no)
 SELECT 'other-intent-'||n,'other-order-'||n,'other-item-'||n,'customer-b' FROM generate_series(1,7) n;
INSERT INTO plugin_idc_core_product_fulfillment_intent (tenant_id,intent_no,order_no,order_item_no,customer_no) VALUES (9,'tenant-hidden','hidden','hidden','customer-a');`)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := newFulfillmentCursorCodec(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	auth := &numberedIntentAuthorizer{allowed: []string{"customer-a"}}
	service := &serviceImpl{database: db, authorizer: auth, cursor: cursor}
	seen := map[string]bool{}
	for _, pageNumber := range []int{1, 2, 3, 50} {
		page, e := service.List(ctx, ListInput{Page: pageNumber, Limit: 10})
		want := 10
		if pageNumber == 3 {
			want = 5
		}
		if pageNumber == 50 {
			want = 0
		}
		if e != nil || page.Total != 25 || len(page.Items) != want {
			t.Fatalf("page%d: %+v %v", pageNumber, page, e)
		}
		for _, item := range page.Items {
			if item.CustomerNo != "customer-a" || seen[item.IntentNo] {
				t.Fatalf("scope or duplicate item %+v", item)
			}
			seen[item.IntentNo] = true
		}
	}
	large, e := service.List(ctx, ListInput{Page: 1, Limit: 100})
	if e != nil || large.Total != 25 || len(large.Items) != 25 {
		t.Fatalf("size100: %+v %v", large, e)
	}
	empty, e := service.List(ctx, ListInput{Page: 2, Limit: 10, CustomerNo: "customer-b"})
	if e != nil || empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("excluded customer %+v %v", empty, e)
	}
	auth.allowed = []string{"customer-b"}
	second, e := service.List(ctx, ListInput{Page: 1, Limit: 10})
	if e != nil || second.Total != 7 || len(second.Items) != 7 {
		t.Fatalf("changed scope %+v %v", second, e)
	}
	auth.denied = true
	if _, e = service.List(ctx, ListInput{Page: 2, Limit: 10}); e != context.Canceled {
		t.Fatalf("denial %v", e)
	}
}
