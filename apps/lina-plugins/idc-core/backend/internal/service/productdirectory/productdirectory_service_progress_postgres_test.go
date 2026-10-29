// This file verifies that service progress is a customer-safe read projection.
// Isolated owner facts exercise manual handling without changing lifecycle or time.

package productdirectory

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestServiceManualProgressPostgreSQL(t *testing.T) {
	var (
		ctx    = context.Background()
		db     = directoryDatabase(t)
		access = &directoryAccessFake{customer: "customer-a"}
	)
	service, err := New(db, access, &directoryAuditFake{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_intent(tenant_id,intent_no,customer_no,status_code,product_name_zh_cn) VALUES
	(0,'manual-a','customer-a','manual_pending','人工处理商品'),
	(0,'queued-a','customer-a','queued','自动开通商品'),
	(0,'incomplete-a','customer-a','incomplete','处理未完成商品'),
	(0,'manual-b','customer-b','incomplete','其他客户秘密商品'),
	(1,'manual-other-tenant','customer-a','manual_pending','其他租户秘密商品');
	INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,product_no,source_intent_no,plugin_id,status_code,order_no) VALUES
	('manual-service','customer-a','product-a','manual-a','idc-server','pending_activation','manual-order'),
	('queued-service','customer-a','product-a','queued-a','idc-server','pending_activation','queued-order'),
	('active-service','customer-a','product-a','manual-a','idc-server','active','active-order'),
	('incomplete-service','customer-a','product-a','incomplete-a','idc-server','pending_activation','incomplete-order'),
	('incomplete-active-service','customer-a','product-a','incomplete-a','idc-server','active','incomplete-active-order'),
	('missing-service','customer-a','product-a','missing-intent','idc-server','pending_activation','missing-order'),
	('cross-customer-service','customer-a','product-a','manual-b','idc-server','pending_activation','cross-order'),
	('cross-tenant-service','customer-a','product-a','manual-other-tenant','idc-server','pending_activation','tenant-order')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_order(tenant_id,order_no,customer_no,paid_at) SELECT tenant_id,order_no,customer_no,CURRENT_TIMESTAMP FROM plugin_idc_core_service_entitlement ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListServices(ctx, AllDirectory, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 10 || len(page.Items) != 10 {
		t.Fatalf("customer total/page: %d/%d", page.Total, len(page.Items))
	}
	items := make(map[string]Entitlement, len(page.Items))
	for _, item := range page.Items {
		items[item.EntitlementNo] = item
		if item.EntitlementNo != "manual-service" && item.EntitlementNo != "incomplete-service" && item.RequiresManualProcessing {
			t.Fatalf("unexpected manual projection: %+v", item)
		}
		if item.ProductNameZhCn == "其他客户秘密商品" || item.ProductNameZhCn == "其他租户秘密商品" {
			t.Fatal("owner name escaped its customer/tenant boundary")
		}
	}
	manual := items["manual-service"]
	if !manual.RequiresManualProcessing || manual.StatusCode != "pending_activation" || manual.OrderNo != "manual-order" || manual.StartedAt > 0 || manual.ExpiresAt != nil {
		t.Fatalf("manual service facts changed: %+v", manual)
	}
	if items["active-service"].StatusCode != "active" {
		t.Fatal("active lifecycle overwritten")
	}
	incomplete := items["incomplete-service"]
	if !incomplete.RequiresManualProcessing || incomplete.StatusCode != "pending_activation" || incomplete.OrderNo != "incomplete-order" || incomplete.StartedAt > 0 || incomplete.ExpiresAt != nil {
		t.Fatalf("incomplete service facts changed: %+v", incomplete)
	}
	if items["incomplete-active-service"].StatusCode != "active" || items["incomplete-active-service"].RequiresManualProcessing {
		t.Fatal("historical incomplete intent overrode active service")
	}
	first, err := service.ListServices(ctx, AllDirectory, 1, 1)
	if err != nil || first.Total != 10 || len(first.Items) != 1 {
		t.Fatalf("bounded page: %+v %v", first, err)
	}
	access.customer = "customer-b"
	other, err := service.ListServices(ctx, AllDirectory, 1, 100)
	if err != nil || other.Total != 1 || len(other.Items) != 1 || other.Items[0].RequiresManualProcessing {
		t.Fatalf("other customer: %+v %v", other, err)
	}
	access.deny = true
	if _, err = service.ListServices(ctx, AllDirectory, 1, 100); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("denied customer read: %v", err)
	}
}

// TestServiceManagementProgressPostgreSQL verifies the extra display progress
// against the same bounded customer page, without rewriting active service facts.
func TestServiceManagementProgressPostgreSQL(t *testing.T) {
	ctx := context.Background()
	db := directoryDatabase(t)
	access := &directoryAccessFake{customer: "customer-a"}
	service, err := New(db, access, &directoryAuditFake{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_entitlement_created_outbox(entitlement_no,intent_no,customer_no,owner_id,status_code)
 VALUES('ent-a','intent-a','customer-a','owner-fixture','pending')`); err != nil {
		t.Fatal(err)
	}
	assertState := func(want string) {
		t.Helper()
		page, readErr := service.ListServices(ctx, AllDirectory, 1, 100)
		if readErr != nil || page.Total != 2 || len(page.Items) != 2 {
			t.Fatalf("page=%+v err=%v", page, readErr)
		}
		found := false
		for _, item := range page.Items {
			if item.EntitlementNo == "ent-a" {
				found = true
				if item.EntitlementOwnerID != "owner-fixture" || item.ManagementPreparation != want || item.StatusCode != "active" || item.StartedAt <= 0 || item.OrderNo != "order-a" {
					t.Fatalf("unexpected management projection: %+v", item)
				}
			} else if item.ManagementPreparation != "" {
				t.Fatal("management progress attached to a different entitlement")
			}
		}
		if !found {
			t.Fatal("missing owned service")
		}
	}
	for state, want := range map[string]string{"pending": "preparing", "retry_wait": "preparing", "delivering": "preparing", "delivered": "ready", "manual_attention": "needs_attention", "private-unknown-state": "needs_attention"} {
		if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code=? WHERE entitlement_no='ent-a'`, state); err != nil {
			t.Fatal(err)
		}
		assertState(want)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='pending_activation' WHERE entitlement_no='ent-a'`); err != nil {
		t.Fatal(err)
	}
	pending, readErr := service.ListServices(ctx, AllDirectory, 1, 100)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, item := range pending.Items {
		if item.EntitlementNo == "ent-a" && (item.StatusCode != "pending_activation" || item.ManagementPreparation != "") {
			t.Fatal("pending service acquired completed-management progress")
		}
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active' WHERE entitlement_no='ent-a'`); err != nil {
		t.Fatal(err)
	}
	for _, mismatch := range []struct{ column, value, original string }{
		{"customer_no", "customer-b", "customer-a"}, {"owner_id", "another-owner", "owner-fixture"}, {"intent_no", "another-intent", "intent-a"},
	} {
		query := "UPDATE plugin_idc_core_entitlement_created_outbox SET " + mismatch.column + "=? WHERE entitlement_no='ent-a'"
		if _, err = db.Exec(ctx, query, mismatch.value); err != nil {
			t.Fatal(err)
		}
		assertState("")
		if _, err = db.Exec(ctx, query, mismatch.original); err != nil {
			t.Fatal(err)
		}
	}
	access.customer = "customer-b"
	page, err := service.ListServices(ctx, AllDirectory, 1, 100)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ManagementPreparation != "" {
		t.Fatalf("foreign projection=%+v err=%v", page, err)
	}
	access.customer = "customer-a"
	if _, err = db.Exec(ctx, `DELETE FROM plugin_idc_core_entitlement_created_outbox WHERE entitlement_no='ent-a'`); err != nil {
		t.Fatal(err)
	}
	assertState("")
}
