// This test proves that a refund sees the committed durable claim after waiting
// for the same parent lock, even though the claim does not change its version.
package billing

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"testing"
	"time"
)

func TestNativeCommercialClaimFencesRefundPostgreSQL(t *testing.T) {
	db := settlementReaderDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no TEXT PRIMARY KEY);
CREATE TABLE plugin_idc_core_business_operation(tenant_id INT,subject_type_code TEXT,subject_no TEXT,resource_owner_code TEXT,operation_type_code TEXT,status_code TEXT,order_no TEXT,provider_task_no TEXT,execution_started_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_native_server_write_fence(tenant_id INT,entitlement_no TEXT,status_code TEXT);
INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'ent_refund_parent');
INSERT INTO plugin_idc_core_business_operation VALUES(0,'entitlement','ent_refund_parent','idc-resource','addon','ready','ord_addon',NULL,NULL);`); err != nil {
		t.Fatal(err)
	}
	s := &onlineRefundService{database: db}
	p := &RefundPreview{OrderNo: "ord_parent", Items: []RefundPreviewItem{{Impacts: []RefundEntitlementImpact{{EntitlementNo: "ent_refund_parent"}}}}}
	locked := make(chan struct{})
	release := make(chan struct{})
	claimDone := make(chan error, 1)
	go func() {
		claimDone <- db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			if _, err := db.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement FOR UPDATE`); err != nil {
				return err
			}
			close(locked)
			<-release
			_, err := db.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code='executing',provider_task_no='resource_task',execution_started_at=CURRENT_TIMESTAMP`)
			return err
		})
	}()
	<-locked
	refundStarted := make(chan struct{})
	refundDone := make(chan error, 1)
	go func() {
		refundDone <- db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			close(refundStarted)
			return s.ensureNativeCommercialRefundIdle(ctx, p)
		})
	}()
	<-refundStarted
	select {
	case err := <-refundDone:
		t.Fatalf("refund escaped parent lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-claimDone; err != nil {
		t.Fatal(err)
	}
	if err := <-refundDone; err == nil {
		t.Fatal("refund missed committed executing state")
	}
	for _, state := range []string{"manual_review", "succeeded"} {
		if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code=?`, state); err != nil {
			t.Fatal(err)
		}
		err := db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return s.ensureNativeCommercialRefundIdle(ctx, p) })
		if (state == "manual_review") != (err != nil) {
			t.Fatalf("state %s returned %v", state, err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_native_server_write_fence VALUES(0,'ent_refund_parent','executing')`); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"executing", "result_unknown", "succeeded", "failed"} {
		if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_native_server_write_fence SET status_code=?`, state); err != nil {
			t.Fatal(err)
		}
		err := db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return s.ensureNativeCommercialRefundIdle(ctx, p) })
		if (state == "executing" || state == "result_unknown") != (err != nil) {
			t.Fatalf("free task fence %s: %v", state, err)
		}
	}
}

func TestNativeRefundOrderInvoiceLockOrderPostgreSQL(t *testing.T) {
	db := settlementReaderDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_commerce_order(tenant_id INT,order_no TEXT PRIMARY KEY,customer_no TEXT);
CREATE TABLE plugin_idc_core_billing_invoice(invoice_no TEXT PRIMARY KEY,tenant_id INT,source_code TEXT,order_no TEXT REFERENCES plugin_idc_core_commerce_order(order_no),customer_no TEXT,currency_code TEXT,status_code TEXT,total_amount_minor BIGINT,credit_amount_minor BIGINT,refundable_amount_minor BIGINT,refunded_amount_minor BIGINT,version BIGINT);
CREATE TABLE test_native_refund(refund_no TEXT,order_no TEXT REFERENCES plugin_idc_core_commerce_order(order_no));
INSERT INTO plugin_idc_core_commerce_order VALUES(0,'ord_lock','cus_lock');
INSERT INTO plugin_idc_core_billing_invoice VALUES('inv_lock',0,'order','ord_lock','cus_lock','CNY','paid',100,0,100,0,1);`); err != nil {
		t.Fatal(err)
	}
	s := &onlineRefundService{database: db}
	locked, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			if _, err := db.GetValue(ctx, `SELECT order_no FROM plugin_idc_core_commerce_order FOR UPDATE`); err != nil {
				return err
			}
			close(locked)
			<-release
			_, err := db.GetValue(ctx, `SELECT invoice_no FROM plugin_idc_core_billing_invoice FOR UPDATE`)
			return err
		})
	}()
	<-locked
	refundDone := make(chan error, 1)
	go func() {
		refundDone <- db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			if _, err := s.loadRefundInvoice(ctx, "cus_lock", "ord_lock", true); err != nil {
				return err
			}
			_, err := db.Exec(ctx, `INSERT INTO test_native_refund VALUES('refund_lock','ord_lock')`)
			return err
		})
	}()
	// The order owner is holding the first lock. The refund must be waiting on
	// that parent rather than retaining the invoice while its FK waits on order.
	select {
	case err := <-refundDone:
		close(release)
		t.Fatalf("refund escaped order lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	probeErr := db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := db.GetValue(ctx, `SELECT invoice_no FROM plugin_idc_core_billing_invoice FOR UPDATE NOWAIT`)
		return err
	})
	close(release)
	if probeErr != nil {
		t.Errorf("refund retained invoice while waiting for order: %v", probeErr)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-refundDone; err != nil {
		t.Fatal(err)
	}
}
