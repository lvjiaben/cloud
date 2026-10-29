// Deletion tests use the real public billing cancellation owner and PostgreSQL
// invoice locks. Payment facts are synthetic; no real balance or provider is used.

package serveroperation

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// nativeCancellationGate pauses before the real public billing cancellation,
// after the lifecycle helper already holds the object and invoice locks.
type nativeCancellationGate struct {
	billing.InvoiceService
	entered, release chan struct{}
	calls            atomic.Int32
}

func (gate *nativeCancellationGate) CancelUnpaid(ctx context.Context, number string) (*billing.Invoice, error) {
	gate.calls.Add(1)
	if gate.entered != nil {
		close(gate.entered)
		select {
		case <-gate.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return gate.InvoiceService.CancelUnpaid(ctx, number)
}

// nativeDeletionBill binds one expired paid object and issues its renewal bill.
func nativeDeletionBill(t *testing.T, database gdb.DB) (servercommercialcap.ObjectRequest, servercommercialcap.Completion, string) {
	t.Helper()
	ctx := context.Background()
	nativeLifecycleIntent(t, database, "create", "native-a", "paid", "succeeded")
	completion := servercommercialcap.Completion{OperationNo: "create", EntitlementNo: "native-a", TaskNo: "resource-task", ObjectNo: "object", Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: strings.Repeat("b", 64), ObservedAt: time.Now().UTC().AddDate(0, -2, 0)}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteCreatedAccessory(ctx, database, completion)
	}); err != nil {
		t.Fatal(err)
	}
	worker := &workerImpl{database: database}
	invoiceNo, _, err := worker.createAccessoryRenewalInvoice(ctx, &dueAccessoryRenewal{EntitlementNo: "accessory-create", CustomerNo: "customer", SubjectTypeCode: "entitlement", SubjectNo: "native-a", CurrencyCode: "CNY", AccessoryKind: "snapshot"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	completion.OperationNo, completion.ObservedAt = "delete", time.Now().UTC()
	return servercommercialcap.ObjectRequest{EntitlementNo: "native-a", CustomerNo: "customer", ObjectNo: "object", Kind: "snapshot"}, completion, invoiceNo
}

func TestNativeLifecycleDeletionPendingInvoicePostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	object, completion, invoice := nativeDeletionBill(t, database)
	invoices, err := billing.NewInvoiceService(database)
	if err != nil {
		t.Fatal(err)
	}
	// A real payment allocation makes public CancelUnpaid refuse this bill.
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_payment_allocation VALUES(0,?,'apply',1)`, invoice); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_billing_invoice SET paid_amount_minor=1 WHERE invoice_no=?`, invoice); err != nil {
		t.Fatal(err)
	}
	pending := false
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var closeErr error
		pending, closeErr = nativeClosePendingRenewalInvoice(ctx, database, invoices, object, completion)
		if closeErr != nil || pending {
			return closeErr
		}
		return nativeCompleteDeletedAccessory(ctx, database, object, completion)
	})
	if err != nil || !pending {
		t.Fatalf("uncertain cancellation was reported completed: pending=%t err=%+v", pending, err)
	}
	kept, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement
WHERE status_code='deletion_pending' AND NOT auto_renew_enabled AND pending_invoice_no=? AND object_no='object'`, invoice)
	if err != nil || kept.Int() != 1 {
		t.Fatal("unresolved bill was detached or kept renewing", err)
	}
	if err = nativeCheckRestore(ctx, database, object); err == nil {
		t.Fatal("deleted object with unresolved bill remained restorable")
	}
	count, reserved, err := nativeAccessoryOccupancy(ctx, database, servercommercialcap.TargetSnapshot{EntitlementNo: "native-a", CustomerNo: "customer"}, "snapshot")
	if err != nil || count != 0 || reserved != 0 {
		t.Fatalf("proven deletion retained an object slot: %d/%d %v", count, reserved, err)
	}
	// A later authoritative payment resolves the money state without refunding
	// it. Retrying the same proven deletion can now close the retained association.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_billing_invoice SET status_code='paid',paid_amount_minor=100,settled_at=CURRENT_TIMESTAMP WHERE invoice_no=?`, invoice); err != nil {
		t.Fatal(err)
	}
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var closeErr error
		pending, closeErr = nativeClosePendingRenewalInvoice(ctx, database, invoices, object, completion)
		if closeErr != nil || pending {
			return closeErr
		}
		return nativeCompleteDeletedAccessory(ctx, database, object, completion)
	})
	if err != nil || pending {
		t.Fatalf("retry did not converge: %t %v", pending, err)
	}
	kept, err = database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_renewal_work_item WHERE status_code='resolved' AND invoice_no=?`, invoice)
	if err != nil || kept.Int() != 1 {
		t.Fatal("resolved deletion left an open work item", err)
	}
}

func TestNativeLifecycleDeletionInvoiceRacePostgreSQL(t *testing.T) {
	for _, first := range []string{"delete", "payment"} {
		t.Run(first, func(t *testing.T) {
			database := nativeLifecycleDB(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			object, completion, invoiceNo := nativeDeletionBill(t, database)
			invoices, err := billing.NewInvoiceService(database)
			if err != nil {
				t.Fatal(err)
			}
			gate := &nativeCancellationGate{InvoiceService: invoices}
			if first == "delete" {
				gate.entered, gate.release = make(chan struct{}), make(chan struct{})
			}
			started, releasePayment := make(chan struct{}), make(chan struct{})
			deleteDone, paymentDone := make(chan error, 1), make(chan error, 1)
			deletePID, paymentPID := make(chan int, 1), make(chan int, 1)
			deleteObject := func() {
				deleteDone <- database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					pid, err := database.GetValue(ctx, `SELECT pg_backend_pid()`)
					if err != nil {
						return err
					}
					deletePID <- pid.Int()
					pending, err := nativeClosePendingRenewalInvoice(ctx, database, gate, object, completion)
					if err != nil {
						return err
					}
					if pending {
						return ErrOperationConflict
					}
					return nativeCompleteDeletedAccessory(ctx, database, object, completion)
				})
			}
			payBill := func() {
				paymentDone <- database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					pid, err := database.GetValue(ctx, `SELECT pg_backend_pid()`)
					if err != nil {
						return err
					}
					paymentPID <- pid.Int()
					if first == "payment" {
						if _, err = database.GetValue(ctx, `SELECT invoice_no FROM plugin_idc_core_billing_invoice WHERE invoice_no=? FOR UPDATE`, invoiceNo); err != nil {
							return err
						}
						close(started)
						select {
						case <-releasePayment:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
					_, err = database.Exec(ctx, `UPDATE plugin_idc_core_billing_invoice SET status_code='paid',paid_amount_minor=100,settled_at=CURRENT_TIMESTAMP WHERE invoice_no=? AND status_code='pending_payment'`, invoiceNo)
					return err
				})
			}
			if first == "delete" {
				go deleteObject()
				<-gate.entered
				go payBill()
				nativeAwaitSQLLock(t, database, <-paymentPID)
				close(gate.release)
			} else {
				go payBill()
				<-started
				go deleteObject()
				nativeAwaitSQLLock(t, database, <-deletePID)
				close(releasePayment)
			}
			if err = <-deleteDone; err != nil {
				t.Fatal(err)
			}
			if err = <-paymentDone; err != nil {
				t.Fatal(err)
			}
			status, err := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_billing_invoice WHERE invoice_no=?`, invoiceNo)
			want := "cancelled"
			if first == "payment" {
				want = "paid"
			}
			if err != nil || status.String() != want {
				t.Fatalf("race result %s want %s: %v", status.String(), want, err)
			}
		})
	}
}

// nativeAwaitSQLLock synchronizes on PostgreSQL's actual lock wait, not a sleep.
func nativeAwaitSQLLock(t *testing.T, database gdb.DB, pid int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		waiting, err := database.GetValue(ctx, `SELECT wait_event_type='Lock' FROM pg_stat_activity WHERE pid=?`, pid)
		if err != nil {
			t.Fatal(err)
		}
		if waiting.Bool() {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("concurrent invoice writer never waited for the authoritative lock")
		}
	}
}
