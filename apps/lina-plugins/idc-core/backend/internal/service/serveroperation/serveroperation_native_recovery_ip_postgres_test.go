// Local IP compensation is tested with a transaction-bound PostgreSQL capacity
// owner substitute. Audit/storage failures must roll back the commercial state;
// the test never contacts the actual provider or modifies shared capacity.

package serveroperation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type nativeRecoveryIPSource struct {
	*nativePricedResourceFixture
	database gdb.DB
	fail     bool
	attempts int
}

func (s *nativeRecoveryIPSource) BindIPCommercial(ctx context.Context, input serveroperationresourcecap.IPCommercialHoldRequest) (*serveroperationresourcecap.IPReservation, error) {
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil || input.OperationNo == "" {
		return nil, ErrOperationConflict
	}
	if err := execOne(ctx, s.database, `UPDATE native_recovery_capacity SET status='commercial_hold',operation_no=? WHERE reservation_no=? AND status='reserved'`, input.OperationNo, input.ReservationNo); err != nil {
		return nil, err
	}
	return &serveroperationresourcecap.IPReservation{ReservationNo: input.ReservationNo, CapacityObservationNo: "capacity", Quantity: 1, Status: serveroperationresourcecap.IPReservationCommercialHold, ExpiresAt: s.reserved[0].ExpiresAt}, nil
}

func (s *nativeRecoveryIPSource) ReleaseIP(context.Context, serveroperationresourcecap.IPReservationReleaseRequest) (*serveroperationresourcecap.IPReservation, error) {
	return nil, serveroperationresourcecap.ErrIPReservationConflict
}

func (s *nativeRecoveryIPSource) ReleaseIPCommercial(ctx context.Context, input serveroperationresourcecap.IPCommercialReleaseRequest) (*serveroperationresourcecap.IPReservation, error) {
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return nil, ErrOperationConflict
	}
	s.attempts++
	if s.fail {
		return nil, ErrServiceUnavailable
	}
	if input.ReasonCode != nativeReadyIPReleaseReason || input.OperationNo == "" {
		return nil, ErrOperationConflict
	}
	if err := execOne(ctx, s.database, `UPDATE native_recovery_capacity SET status='released',release_count=release_count+1 WHERE reservation_no=? AND operation_no=? AND status='commercial_hold'`, input.ReservationNo, input.OperationNo); err != nil {
		return nil, err
	}
	return &serveroperationresourcecap.IPReservation{ReservationNo: input.ReservationNo, Status: serveroperationresourcecap.IPReservationReleased, ExpiresAt: time.Now().UTC().Add(time.Hour)}, nil
}

func TestNativeReadyIPRefundAtomicRecoveryPostgreSQL(t *testing.T) {
	f, source := newNativePricedFixture(t, 1000)
	if _, err := f.database.Exec(f.ctx, `CREATE TABLE native_recovery_capacity(reservation_no TEXT PRIMARY KEY,status TEXT,release_count INT DEFAULT 0,operation_no TEXT);
INSERT INTO native_recovery_capacity VALUES('reservation','reserved',0,NULL)`); err != nil {
		t.Fatal(err)
	}
	capacity := &nativeRecoveryIPSource{nativePricedResourceFixture: source, database: f.database}
	f.native.prices.source = capacity
	_, p := nativePricedIPCheckout(t, f)
	f.assertCount(t, `SELECT count(*) FROM native_recovery_capacity WHERE status='commercial_hold' AND operation_no IS NOT NULL`, 1)
	nativeRecoverySettle(t, f, p, false)
	nativeRecoveryRefund(t, f, p, p.TotalAmountMinor, false)
	if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 1 {
		t.Fatalf("pending hold n=%d error=%+v", n, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review' AND execution_started_at IS NULL AND provider_task_no IS NULL`, 1)
	nativeRecoveryRunRefund(t, f)
	if _, err := f.database.Exec(f.ctx, `CREATE FUNCTION native_recovery_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.reason_code='server_commercial.refund_cancelled' THEN RAISE EXCEPTION 'injected recovery audit failure'; END IF; RETURN NEW; END $$;
CREATE TRIGGER native_recovery_fail_audit BEFORE INSERT ON plugin_idc_core_business_operation_event FOR EACH ROW EXECUTE FUNCTION native_recovery_fail_audit()`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.native.ReconcileReady(f.ctx, 10); err == nil {
		t.Fatal("audit failure committed cancellation")
	}
	if capacity.attempts != 0 {
		t.Fatal("capacity was released before confirmation audit")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review'`, 1)
	if _, err := f.database.Exec(f.ctx, `DROP TRIGGER native_recovery_fail_audit ON plugin_idc_core_business_operation_event`); err != nil {
		t.Fatal(err)
	}
	capacity.fail = true
	if _, err := f.native.ReconcileReady(f.ctx, 10); err == nil {
		t.Fatal("capacity failure committed cancellation")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation_event WHERE reason_code='server_commercial.refund_cancelled'`, 0)
	capacity.fail = false
	var wait sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() { defer wait.Done(); _, err := f.native.ReconcileReady(f.ctx, 10); errs <- err }()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if capacity.attempts != 2 {
		t.Fatalf("release attempts=%d want one failure and one success", capacity.attempts)
	}
	f.assertCount(t, `SELECT count(*) FROM native_recovery_capacity WHERE status='released' AND release_count=1`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='cancelled' AND execution_started_at IS NULL AND provider_task_no IS NULL`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation_event WHERE reason_code='server_commercial.refund_cancelled'`, 1)
}
