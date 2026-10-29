// A real paid native checkout must replay while its settlement delivery waits
// for the same parent. This catches operation-before-parent lock inversion.
package serveroperation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

func TestNativeSettlementLocksParentBeforeOperationPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	q, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionTrafficAdd, PackageNo: "POL_checkout_test", IdempotencyKey: "native-lock-quote"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: q.QuoteNo, IdempotencyKey: "native-lock-checkout"})
	if err != nil {
		t.Fatal(err)
	}
	f.payNativeInvoice(t, p.InvoiceNo, p.TotalAmountMinor)
	settled, err := f.settlementService(t).SettleOrder(f.ctx, p.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	var event outbox.Event
	if err = f.database.GetScan(f.ctx, &event, `SELECT event_no,event_type,event_version,topic,aggregate_type,aggregate_no,correlation_no,payload,metadata FROM plugin_idc_core_outbox_event WHERE aggregate_no=? AND correlation_no=?`, p.OrderNo, settled.SettlementNo); err != nil {
		t.Fatal(err)
	}
	addon, err := NewAccessorySettlementConsumer(f.database, f.native.evidence)
	if err != nil {
		t.Fatal(err)
	}
	router, err := fulfillment.NewRoutedConsumer(&accessoryChainGeneric{}, addon)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	started := false
	err = f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := f.native.readNativeCommercialTarget(ctx, f.target, true); readErr != nil {
			return readErr
		}
		pid, readErr := f.database.GetValue(ctx, `SELECT pg_backend_pid()`)
		if readErr != nil {
			return readErr
		}
		started = true
		go func() {
			_, consumeErr := router.Consume(f.ctx, outbox.ClaimedEvent{Event: event})
			result <- consumeErr
		}()
		deadline := time.Now().Add(10 * time.Second)
		for {
			blocked, waitErr := f.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE NOT granted AND ?=ANY(pg_blocking_pids(pid)))`, pid.Int())
			if waitErr != nil {
				return waitErr
			}
			if blocked.Bool() {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("settlement did not wait on parent")
			}
			time.Sleep(10 * time.Millisecond)
		}
		_, readErr = f.database.GetValue(ctx, `SELECT operation_no FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND operation_no=? FOR UPDATE NOWAIT`, p.OperationNo)
		return readErr
	})
	if started {
		select {
		case consumeErr := <-result:
			if consumeErr != nil {
				t.Fatalf("settlement after parent unlock: %+v", consumeErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("settlement remained blocked")
		}
	}
	if err != nil {
		t.Fatalf("settlement locked operation before parent, blocking checkout replay: %+v", err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='ready'`, 1)
}
