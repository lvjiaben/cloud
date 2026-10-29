// Native IP commercial tests start with a genuinely paid generic entitlement
// and exercise actual migrations, checkout, settlement and resource admission.

package serveroperation

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type nativePricedResourceFixture struct {
	nativePriceSourceFixture
	consumed, releases, held            int
	commercialBinds, commercialReleases int
	commercialOperationNo               string
	commercialBindFailure               bool
}

func (f *nativePricedResourceFixture) BindIPCommercial(_ context.Context, input serveroperationresourcecap.IPCommercialHoldRequest) (*serveroperationresourcecap.IPReservation, error) {
	if f.commercialBindFailure {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	if f.commercialOperationNo != "" && f.commercialOperationNo != input.OperationNo {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	f.commercialBinds++
	f.commercialOperationNo = input.OperationNo
	return &serveroperationresourcecap.IPReservation{ReservationNo: input.ReservationNo, CapacityObservationNo: "capacity", Quantity: 1, Status: serveroperationresourcecap.IPReservationCommercialHold, ExpiresAt: f.reserved[0].ExpiresAt}, nil
}

func TestNativePricedIPCommercialHoldFailureRollsBackPostgreSQL(t *testing.T) {
	f, source := newNativePricedFixture(t, 1000)
	q, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionChangeIP, IdempotencyKey: "native-priced-hold-rollback"})
	if err != nil {
		t.Fatal(err)
	}
	source.commercialBindFailure = true
	input := servercommercialcap.CheckoutRequest{QuoteNo: q.QuoteNo, Target: f.target, IdempotencyKey: "native-priced-hold-rollback"}
	if _, err := f.native.Checkout(f.ctx, input); err == nil {
		t.Fatal("checkout accepted a failed commercial capacity hold")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE quote_no='`+q.QuoteNo+`'`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_order WHERE quote_no='`+q.QuoteNo+`'`, 0)
	source.commercialBindFailure = false
	p, err := f.native.Checkout(f.ctx, input)
	if err != nil || p == nil || source.commercialOperationNo != p.OperationNo || source.commercialBinds != 1 {
		t.Fatalf("rolled back checkout cannot retry: %+v %v", p, err)
	}
}

func (f *nativePricedResourceFixture) ReleaseIPCommercial(_ context.Context, input serveroperationresourcecap.IPCommercialReleaseRequest) (*serveroperationresourcecap.IPReservation, error) {
	if f.commercialOperationNo != input.OperationNo || input.ReasonCode != "core_order_cancelled" {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	f.commercialReleases++
	return &serveroperationresourcecap.IPReservation{ReservationNo: input.ReservationNo, CapacityObservationNo: "capacity", Quantity: 1, Status: serveroperationresourcecap.IPReservationReleased, ExpiresAt: f.reserved[0].ExpiresAt}, nil
}

func (f *nativePricedResourceFixture) HoldIP(_ context.Context, input serveroperationresourcecap.IPReservationHoldRequest) (*serveroperationresourcecap.IPReservation, error) {
	f.held++
	return &serveroperationresourcecap.IPReservation{ReservationNo: input.ReservationNo, CapacityObservationNo: "capacity", Quantity: 1, Status: serveroperationresourcecap.IPReservationInFlight, ExpiresAt: f.reserved[0].ExpiresAt}, nil
}

func (f *nativePricedResourceFixture) ConsumeIP(_ context.Context, input serveroperationresourcecap.IPReservationConsumeRequest) (*serveroperationresourcecap.IPReservation, error) {
	f.consumed++
	return &serveroperationresourcecap.IPReservation{ReservationNo: input.ReservationNo, CapacityObservationNo: "capacity", Quantity: 1, Status: serveroperationresourcecap.IPReservationConsumed, ExpiresAt: f.reserved[0].ExpiresAt}, nil
}
func (f *nativePricedResourceFixture) ReleaseIP(_ context.Context, input serveroperationresourcecap.IPReservationReleaseRequest) (*serveroperationresourcecap.IPReservation, error) {
	f.releases++
	return &serveroperationresourcecap.IPReservation{ReservationNo: input.ReservationNo, CapacityObservationNo: "capacity", Quantity: 1, Status: serveroperationresourcecap.IPReservationReleased, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func newNativePricedFixture(t *testing.T, amount int64) (nativeCommercialFixture, *nativePricedResourceFixture) {
	t.Helper()
	f := newNativeCommercialFixture(t)
	for _, name := range []string{"152-native-priced-server-operation.sql"} {
		nativeLifecycleMigration(t, f.database, name)
	}
	if err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := f.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_ip_change_policy(policy_no,currency_code,current_version) VALUES('IP_native','CNY',1)`); err != nil {
			return err
		}
		_, err := f.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_ip_change_policy_version(policy_no,policy_version,currency_code,enabled,free_changes_per_window,window_days,max_changes_per_day,amount_minor,maximum_snapshot_age_seconds,content_digest,published_by) VALUES('IP_native',1,'CNY',TRUE,0,30,3,?,300,repeat('a',64),101)`, amount)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	f.target.Items = []serveroperationresourcecap.AccessoryItem{{ObjectNo: "IP_local", Kind: serveroperationresourcecap.AccessoryKindIP, State: "active", Projection: json.RawMessage(`{"address":"192.0.2.1"}`)}}
	source := &nativePricedResourceFixture{nativePriceSourceFixture: nativePriceSourceFixture{projection: &serveroperationresourcecap.AccessoryProjection{ServiceNo: f.target.LocalResourceNo, SnapshotNo: f.target.SnapshotNo, ObservedAt: f.target.ObservedAt, ExpiresAt: f.target.ExpiresAt, Actions: f.target.Actions, Items: f.target.Items}}}
	f.native.prices.source = source
	f.native.pricing.parameters = source
	return f, source
}

func nativePricedIPCheckout(t *testing.T, f nativeCommercialFixture) (*servercommercialcap.Quote, *servercommercialcap.Purchase) {
	t.Helper()
	q, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionChangeIP, IdempotencyKey: "native-priced-quote"})
	if err != nil {
		t.Fatalf("IP quote: %+v", err)
	}
	p, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{QuoteNo: q.QuoteNo, Target: f.target, IdempotencyKey: "native-priced-checkout"})
	if err != nil {
		t.Fatalf("IP checkout: %+v", err)
	}
	return q, p
}

func TestNativePricedIPFreeAndTaskAdmissionPostgreSQL(t *testing.T) {
	f, source := newNativePricedFixture(t, 0)
	q, p := nativePricedIPCheckout(t, f)
	if source.commercialBinds != 1 || source.commercialOperationNo != p.OperationNo {
		t.Fatal("checkout did not bind capacity to its actual commercial intent")
	}
	if q.TotalAmountMinor != 0 || p.Status != "ready" || p.InvoiceNo != "" || p.OrderNo != "" {
		t.Fatalf("free IP fabricated payment %+v %+v", q, p)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_channel_service_quote WHERE source_code='native' AND channel_id IS NULL`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_channel_service_operation`, 0)
	// These are otherwise valid ready-to-executing writes, with one frozen
	// coordinate changed. Constraint-only unresolved rows would miss this bypass.
	frozen, err := f.native.loadNativePricedQuote(f.ctx, q.QuoteNo, f.target.CustomerNo, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"route", "evidence", "version"} {
		t.Run("reject_"+name, func(t *testing.T) {
			channel, digest, version := f.target.ChannelNo, frozen.ContentDigest, f.target.EntitlementVersion
			switch name {
			case "route":
				channel = "foreign-channel"
			case "evidence":
				digest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			case "version":
				version++
			}
			err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
				_, err := f.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code='executing',execution_mode_code='provider_confirmed',channel_no=?,capability_evidence_no=?,capability_evidence_digest=?,expected_subject_version=?,provider_task_no='tampered-task',version=version+1 WHERE operation_no=?`, channel, q.QuoteNo, digest, version, p.OperationNo)
				return err
			})
			if err == nil {
				t.Fatal("tampered native binding accepted")
			}
		})
	}
	t.Run("reject_non_native_quote", func(t *testing.T) {
		reached := false
		err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
			if _, err := f.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation(tenant_id,operation_no,operation_type_code,customer_no,service_no,subject_type_code,subject_no,currency_code,status_code,financial_settlement_required,execution_mode_code,resource_owner_code,pool_no,idempotency_digest,request_digest,content_digest,expected_subject_version,last_error_code,version) VALUES(0,'unpriced-native-probe','start',?,NULL,'entitlement',?,?,'ready',FALSE,'unresolved','idc-resource',?,repeat('1',64),repeat('2',64),repeat('3',64),?,'',1)`, f.target.CustomerNo, f.target.EntitlementNo, q.CurrencyCode, f.target.PoolNo, f.target.EntitlementVersion); err != nil {
				return err
			}
			reached = true
			_, err := f.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code='executing',execution_mode_code='provider_confirmed',channel_no=?,capability_evidence_no=?,capability_evidence_digest=?,provider_task_no='tampered-task',version=version+1 WHERE operation_no='unpriced-native-probe'`, f.target.ChannelNo, q.QuoteNo, frozen.ContentDigest)
			return err
		})
		if !reached || err == nil {
			t.Fatalf("non-quote binding probe reached=%v err=%v", reached, err)
		}
	})
	called := 0
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(_ context.Context, op servercommercialcap.Operation) (string, error) {
		called++
		if op.EntitlementNo != f.target.EntitlementNo || op.ResourceReservationNo != "reservation" || op.Action != serveroperationresourcecap.ActionChangeIP {
			return "", ErrOperationConflict
		}
		return "resource-task-ip", nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_business_operation SET provider_task_no='replacement-task' WHERE operation_no=?`, p.OperationNo); err == nil {
		t.Fatal("existing native task replaced")
	}
	if called != 1 || source.consumed != 0 || source.held != 1 {
		t.Fatal("task admission consumed capacity before provider success")
	}
	if source.commercialBinds != 2 || len(source.reserved) != 2 {
		t.Fatal("execution renewed a quote reservation instead of rechecking its commercial hold")
	}
	if err := f.native.WithQueuedOperation(f.ctx, p.OperationNo, "resource-task-ip", func(context.Context, servercommercialcap.Operation) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if source.consumed != 0 || source.held != 1 {
		t.Fatal("queued task consumed or rebound capacity before provider success")
	}
	completion := servercommercialcap.Completion{OperationNo: p.OperationNo, EntitlementNo: f.target.EntitlementNo, TaskNo: "resource-task-ip", EvidenceDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ObservedAt: time.Now().UTC(), Status: servercommercialcap.CompletionUnknown}
	if err := f.native.Complete(f.ctx, completion); err != nil {
		t.Fatal(err)
	}
	if source.consumed != 0 || source.releases != 0 {
		t.Fatal("unknown result released or consumed capacity")
	}
	completion.Status = servercommercialcap.CompletionSucceeded
	if err := f.native.Complete(f.ctx, completion); err == nil {
		t.Fatal("unchanged old IP set was accepted as success")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review'`, 1)
	source.projection.Items[0].Projection = json.RawMessage(`{"address":"192.0.2.2"}`)
	source.projection.ObservedAt = time.Now().UTC()
	if err := f.native.Complete(f.ctx, completion); err != nil {
		t.Fatal(err)
	}
	if source.consumed != 1 {
		t.Fatal("proven new addresses did not consume reservation")
	}
	if err := f.native.Complete(f.ctx, completion); err != nil || source.consumed != 1 {
		t.Fatal("completion replay consumed twice")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_instance`, 0)
}

func TestNativePricedIPPaidFinancialSettlementPostgreSQL(t *testing.T) {
	f, _ := newNativePricedFixture(t, 1000)
	_, p := nativePricedIPCheckout(t, f)
	if p.Status != "awaiting_financial_settlement" {
		t.Fatalf("unpaid IP ready %+v", p)
	}
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) {
		t.Fatal("unpaid operation reached resource")
		return "", nil
	}); err == nil {
		t.Fatal("unpaid execution admitted")
	}
	if _, err := f.settlementService(t).SettleOrder(f.ctx, p.OrderNo); err == nil {
		t.Fatal("unpaid IP settled")
	}
	f.payNativeInvoice(t, p.InvoiceNo, p.TotalAmountMinor)
	// Invoice payment alone still cannot dispatch a resource operation.
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) {
		t.Fatal("invoice-only operation reached resource")
		return "", nil
	}); err == nil {
		t.Fatal("invoice paid treated as financially settled")
	}
	settled, err := f.settlementService(t).SettleOrder(f.ctx, p.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := billing.NewSettlementEvidenceReader(f.database)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewServerOperationSettlementConsumer(f.database, evidence)
	if err != nil {
		t.Fatal(err)
	}
	generic := &accessoryChainGeneric{}
	router, err := fulfillment.NewRoutedConsumer(generic, consumer)
	if err != nil {
		t.Fatal(err)
	}
	var event outbox.Event
	if err = f.database.GetScan(f.ctx, &event, `SELECT event_no,event_type,event_version,topic,aggregate_type,aggregate_no,correlation_no,payload,metadata FROM plugin_idc_core_outbox_event WHERE aggregate_no=? AND correlation_no=?`, p.OrderNo, settled.SettlementNo); err != nil {
		t.Fatal(err)
	}
	wrong := event
	wrong.Payload = append(json.RawMessage(nil), event.Payload...)
	var payload map[string]any
	if err = json.Unmarshal(wrong.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["totalAmountMinor"] = p.TotalAmountMinor + 1
	wrong.Payload, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = router.Consume(f.ctx, outbox.ClaimedEvent{Event: wrong}); err == nil {
		t.Fatal("changed settlement envelope accepted")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_financial_settlement_delivery`, 0)
	var wait sync.WaitGroup
	failures := make(chan error, 3)
	for range 3 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := router.Consume(f.ctx, outbox.ClaimedEvent{Event: event})
			failures <- err
		}()
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if generic.calls != 0 {
		t.Fatal("native IP reached generic new purchase")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_financial_settlement_delivery`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_type_code='ip_change' AND status_code='ready' AND financially_settled_at IS NOT NULL`, 1)
	if err = f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) { return "paid-ip-task", nil }); err != nil {
		t.Fatal(err)
	}
	if _, err = router.Consume(f.ctx, outbox.ClaimedEvent{Event: event}); err != nil {
		t.Fatalf("settlement replay after task binding: %v", err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='executing'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_instance`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_operation_execution`, 0)
}

func TestNativePricedIPCompletedCancellationPostgreSQL(t *testing.T) {
	f, source := newNativePricedFixture(t, 1000)
	_, p := nativePricedIPCheckout(t, f)
	owner, err := commerce.New(f.database, cancellationCatalog{}, cancellationPrincipal{}, f.settlementService(t))
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := owner.CancelAccountOrder(f.ctx, commerce.OrderCancellationRequest{OrderNo: p.OrderNo, IdempotencyKey: "native-priced-cancel"})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := commerce.NewCancellationEvidenceReader(f.database)
	if err != nil {
		t.Fatal(err)
	}
	absent, err := NewServerOperationCancellationConsumer(f.database, reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	input := productfulfillment.OrderCancellationInput{OrderNo: p.OrderNo, CancellationNo: cancelled.CancellationNo}
	if _, handled, err := absent.ConsumeAddonCancellation(f.ctx, input); err == nil || !handled {
		t.Fatal("missing resource capacity was ignored or fell through")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='awaiting_financial_settlement'`, 1)
	consumer, err := NewServerOperationCancellationConsumer(f.database, reader, source)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		result, handled, consumeErr := consumer.ConsumeAddonCancellation(f.ctx, input)
		if consumeErr != nil || !handled || result.Cancelled != 1-i {
			t.Fatalf("cancel replay %+v %t %v", result, handled, consumeErr)
		}
	}
	if source.commercialReleases != 2 || source.releases != 0 {
		t.Fatal("idempotent cancellation did not release same reservation")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='cancelled'`, 1)
}

func TestNativePricedIPDefinitiveFailureReleasesHeldCapacityPostgreSQL(t *testing.T) {
	f, source := newNativePricedFixture(t, 0)
	_, p := nativePricedIPCheckout(t, f)
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) { return "failed-ip-task", nil }); err != nil {
		t.Fatal(err)
	}
	if source.held != 1 || source.consumed != 0 {
		t.Fatal("task did not preserve held capacity")
	}
	input := servercommercialcap.Completion{OperationNo: p.OperationNo, EntitlementNo: f.target.EntitlementNo, TaskNo: "failed-ip-task", EvidenceDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ObservedAt: time.Now().UTC(), Status: servercommercialcap.CompletionFailed}
	for range 2 {
		if err := f.native.Complete(f.ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	if source.releases != 1 || source.consumed != 0 {
		t.Fatal("definitive failure leaked or consumed capacity")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_type_code='ip_change' AND status_code='failed'`, 1)
}
