package customeroperations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func TestNativeIPHoldSurvivesExpiryAndUncertainTaskPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	if _, err := f.db.Exec(f.ctx, `INSERT INTO plugin_idc_resource_provider_capacity_observation(observation_no,channel_no,capacity_kind,status_code,item_count,observed_at,completed_at) VALUES('stock-main','channel-main','stock','complete',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_resource_provider_capacity_observation_item(observation_no,capacity_kind,external_ref,projection,observed_at) VALUES('stock-main','stock','1','{"ip_free":1}',CURRENT_TIMESTAMP);
UPDATE plugin_idc_resource_task SET capability_code='server.ip.change',status_code='pending',finished_at=NULL WHERE task_no='task-1'`); err != nil {
		t.Fatal(err)
	}
	request := serveroperationresourcecap.IPReservationRequest{Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", PoolNo: "pool-main"}, Quantity: 1, MaximumCapacityAgeSeconds: 300, ExpiresAt: time.Now().Add(time.Minute), IdempotencyKey: "native-ip-hold-first"}
	var err error
	request.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := f.source.ReserveIP(f.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	commercial := serveroperationresourcecap.IPCommercialHoldRequest{ReservationNo: reservation.ReservationNo, OperationNo: "commercial-ip-1"}
	checkoutRollback := errors.New("checkout rollback")
	err = f.db.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, bindErr := f.source.BindIPCommercial(ctx, commercial); bindErr != nil {
			return bindErr
		}
		return checkoutRollback
	})
	if !errors.Is(err, checkoutRollback) {
		t.Fatal(err)
	}
	if held, bindErr := f.source.BindIPCommercial(f.ctx, commercial); bindErr != nil || held.Reused || held.Status != serveroperationresourcecap.IPReservationCommercialHold {
		t.Fatal("checkout hold rollback/rebind", bindErr)
	}
	if _, err = f.source.ReleaseIP(f.ctx, serveroperationresourcecap.IPReservationReleaseRequest{ReservationNo: reservation.ReservationNo, ReasonCode: "quote_expired"}); err == nil {
		t.Fatal("ordinary quote expiry released checkout hold")
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_ip_reservation SET created_at=CURRENT_TIMESTAMP-INTERVAL '2 minutes',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 minute'`); err != nil {
		t.Fatal(err)
	}
	competing := request
	competing.IdempotencyKey = "native-ip-checkout-competitor"
	competing.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(competing)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.source.ReserveIP(f.ctx, competing); !errors.Is(err, serveroperationresourcecap.ErrIPCapacityUnavailable) {
		t.Fatal("expired checkout hold resold", err)
	}
	hold := serveroperationresourcecap.IPReservationHoldRequest{ReservationNo: reservation.ReservationNo, ProviderTaskNo: "task-1"}
	if _, err = f.source.HoldIP(f.ctx, hold); err == nil {
		t.Fatal("task from another commercial intent admitted")
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET owner_metadata='{"operationNo":"commercial-ip-1"}' WHERE task_no='task-1'`); err != nil {
		t.Fatal(err)
	}
	rolledBack := errors.New("enclosing business transaction failed")
	err = f.db.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, holdErr := f.source.HoldIP(ctx, hold); holdErr != nil {
			return holdErr
		}
		return rolledBack
	})
	if !errors.Is(err, rolledBack) {
		t.Fatal(err)
	}
	state, err := f.db.GetValue(f.ctx, `SELECT status_code FROM plugin_idc_resource_ip_reservation WHERE reservation_no=?`, reservation.ReservationNo)
	if err != nil || state.String() != "commercial_hold" {
		t.Fatal("hold escaped owning transaction", err)
	}
	held, err := f.source.HoldIP(f.ctx, hold)
	if err != nil || held.Status != serveroperationresourcecap.IPReservationInFlight {
		t.Fatalf("hold: %+v %v", held, err)
	}
	if replay, replayErr := f.source.HoldIP(f.ctx, hold); replayErr != nil || !replay.Reused {
		t.Fatal("hold replay failed", replayErr)
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_ip_reservation SET created_at=CURRENT_TIMESTAMP-INTERVAL '2 minutes',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 minute';UPDATE plugin_idc_resource_task SET status_code='result_unknown',result_unknown=true WHERE task_no='task-1'`); err != nil {
		t.Fatal(err)
	}
	request.IdempotencyKey = "native-ip-hold-second"
	request.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.source.ReserveIP(f.ctx, request); !errors.Is(err, serveroperationresourcecap.ErrIPCapacityUnavailable) {
		t.Fatal("expired in-flight capacity was resold", err)
	}
	release := serveroperationresourcecap.IPReservationReleaseRequest{ReservationNo: reservation.ReservationNo, ReasonCode: "confirmed_failed"}
	if _, err = f.source.ReleaseIP(f.ctx, release); err == nil {
		t.Fatal("unknown result released IP")
	}
	if _, err = f.source.ConsumeIP(f.ctx, serveroperationresourcecap.IPReservationConsumeRequest{ReservationNo: reservation.ReservationNo, ProviderTaskNo: "task-1"}); err == nil {
		t.Fatal("unknown result consumed IP")
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='failed',result_unknown=false,finished_at=CURRENT_TIMESTAMP WHERE task_no='task-1'`); err != nil {
		t.Fatal(err)
	}
	if released, releaseErr := f.source.ReleaseIP(f.ctx, release); releaseErr != nil || released.Status != serveroperationresourcecap.IPReservationReleased {
		t.Fatal("proven failure did not release", releaseErr)
	}
	if _, err = f.source.ReserveIP(f.ctx, request); err != nil {
		t.Fatal("released capacity stayed occupied", err)
	}
	second, err := f.source.ReserveIP(f.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	commercial = serveroperationresourcecap.IPCommercialHoldRequest{ReservationNo: second.ReservationNo, OperationNo: "commercial-ip-2"}
	if _, err = f.source.BindIPCommercial(f.ctx, commercial); err != nil {
		t.Fatal(err)
	}
	closing := serveroperationresourcecap.IPCommercialReleaseRequest{ReservationNo: second.ReservationNo, OperationNo: "wrong-intent", ReasonCode: "core_order_cancelled"}
	if _, err = f.source.ReleaseIPCommercial(f.ctx, closing); err == nil {
		t.Fatal("foreign intent released hold")
	}
	closing.OperationNo = commercial.OperationNo
	closing.ReasonCode = "refund_pending"
	if _, err = f.source.ReleaseIPCommercial(f.ctx, closing); err == nil {
		t.Fatal("pending refund released hold")
	}
	closing.ReasonCode = "core_order_cancelled"
	if released, closeErr := f.source.ReleaseIPCommercial(f.ctx, closing); closeErr != nil || released.Status != serveroperationresourcecap.IPReservationReleased {
		t.Fatal("completed cancellation release", closeErr)
	}
}
