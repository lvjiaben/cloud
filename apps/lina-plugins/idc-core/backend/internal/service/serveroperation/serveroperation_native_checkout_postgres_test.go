// These tests prove native addon commercial admission against production foreign
// keys and the real financial/cancellation evidence owners, without provider IO.
package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func TestNativeCheckoutFreeSnapshotPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	in := servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionSnapshotCreate, Parameters: json.RawMessage(`{"name":"test-native-snapshot"}`), IdempotencyKey: "native-free-snapshot"}
	quote, err := f.native.Quote(f.ctx, in)
	if err != nil {
		t.Fatalf("native free quote: %+v", err)
	}
	if quote.TotalAmountMinor != 0 || quote.ChargedQuantity != 0 || quote.FreeCountPerServer != 1 || quote.EntitlementNo != f.target.EntitlementNo {
		t.Fatalf("wrong free quote: %+v", quote)
	}
	replay, err := f.native.Quote(f.ctx, in)
	if err != nil || replay.QuoteNo != quote.QuoteNo || !replay.Reused {
		t.Fatalf("quote replay=%+v / %+v", replay, err)
	}
	checkout := servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: quote.QuoteNo, IdempotencyKey: "native-free-checkout"}
	purchase, err := f.native.Checkout(f.ctx, checkout)
	if err != nil {
		t.Fatalf("native free checkout: %+v", err)
	}
	if purchase.Status != "ready" || purchase.OrderNo != "" || purchase.InvoiceNo != "" {
		t.Fatalf("free operation created financial evidence: %+v", purchase)
	}
	replayPurchase, err := f.native.Checkout(f.ctx, checkout)
	if err != nil || !replayPurchase.Reused || replayPurchase.OperationNo != purchase.OperationNo {
		t.Fatalf("purchase replay=%+v / %+v", replayPurchase, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE subject_type_code='entitlement' AND service_no IS NULL AND expected_service_version IS NULL AND execution_mode_code='unresolved' AND channel_no IS NULL AND status_code='ready'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE subject_type_code='entitlement' AND service_no IS NULL AND status_code='pending_binding' AND billing_mode_code='free'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_instance`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_operation_execution`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_financial_settlement`, 1)
}

func TestNativeCheckoutPaidFinancialChainPostgreSQL(t *testing.T) {
	for _, action := range []serveroperationresourcecap.Action{serveroperationresourcecap.ActionTrafficAdd, serveroperationresourcecap.ActionBackupCreate} {
		t.Run(string(action), func(t *testing.T) {
			f := newNativeCommercialFixture(t)
			if action == serveroperationresourcecap.ActionBackupCreate {
				if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_catalog_tax_policy SET tax_treatment_code='taxable',tax_rate_basis_points=1900 WHERE tax_policy_no='TAX_checkout_test'`); err != nil {
					t.Fatal(err)
				}
			}
			input := servercommercialcap.QuoteRequest{Target: f.target, Action: action, IdempotencyKey: "native-paid-quote"}
			if action == serveroperationresourcecap.ActionTrafficAdd {
				input.PackageNo = "POL_checkout_test"
			} else {
				input.Parameters = json.RawMessage(`{"name":"test-native-backup"}`)
			}
			quote, err := f.native.Quote(f.ctx, input)
			if err != nil {
				t.Fatalf("native paid quote: %+v", err)
			}
			if action == serveroperationresourcecap.ActionBackupCreate && quote.TaxAmountMinor != 160 {
				t.Fatalf("inclusive tax=%+v", quote)
			}
			if quote.TotalAmountMinor != 1000 {
				t.Fatalf("amount=%+v", quote)
			}
			p, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: quote.QuoteNo, IdempotencyKey: "native-paid-checkout"})
			if err != nil {
				t.Fatalf("native paid checkout: %+v", err)
			}
			if p.Status != "awaiting_financial_settlement" {
				t.Fatalf("unpaid started: %+v", p)
			}
			if _, err = f.settlementService(t).SettleOrder(f.ctx, p.OrderNo); err == nil {
				t.Fatal("unpaid native operation settled")
			}
			f.payNativeInvoice(t, p.InvoiceNo, p.TotalAmountMinor)
			settlement, err := f.settlementService(t).SettleOrder(f.ctx, p.OrderNo)
			if err != nil {
				t.Fatalf("native settle: %+v", err)
			}
			evidence, err := billing.NewSettlementEvidenceReader(f.database)
			if err != nil {
				t.Fatal(err)
			}
			addon, err := NewAccessorySettlementConsumer(f.database, evidence)
			if err != nil {
				t.Fatal(err)
			}
			generic := &accessoryChainGeneric{}
			router, err := fulfillment.NewRoutedConsumer(generic, addon)
			if err != nil {
				t.Fatal(err)
			}
			var event outbox.Event
			if err = f.database.GetScan(f.ctx, &event, `SELECT event_no,event_type,event_version,topic,aggregate_type,aggregate_no,correlation_no,payload,metadata FROM plugin_idc_core_outbox_event WHERE aggregate_no=? AND correlation_no=?`, p.OrderNo, settlement.SettlementNo); err != nil {
				t.Fatal(err)
			}
			const workers = 4
			var wait sync.WaitGroup
			errs := make(chan error, workers)
			for range workers {
				wait.Add(1)
				go func() {
					defer wait.Done()
					_, err := router.Consume(f.ctx, outbox.ClaimedEvent{Event: event})
					errs <- err
				}()
			}
			wait.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatalf("native real consumer: %+v", err)
				}
			}
			if generic.calls != 0 {
				t.Fatal("native addon reached generic purchase")
			}
			f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='ready' AND financially_settled_at IS NOT NULL AND subject_type_code='entitlement' AND channel_no IS NULL`, 1)
			f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_instance`, 0)
			f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_operation_execution`, 0)
			f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE version=1`, 1)
			f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_financial_settlement_delivery`, 1)
		})
	}
}

func TestNativeCheckoutCancellationStockPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	packages, err := f.native.ListTrafficPackages(f.ctx, f.target)
	if err != nil || len(packages) != 1 || packages[0].RemainingStock == nil || *packages[0].RemainingStock != 1 {
		t.Fatalf("initial packages=%+v / %+v", packages, err)
	}
	q, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionTrafficAdd, PackageNo: "POL_checkout_test", IdempotencyKey: "native-stock-quote"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: q.QuoteNo, IdempotencyKey: "native-stock-checkout"})
	if err != nil {
		t.Fatal(err)
	}
	assertTrafficPackageOccupancy(t, f.accessoryCheckoutFixture, 1)
	owner, err := commerce.New(f.database, cancellationCatalog{}, cancellationPrincipal{}, f.settlementService(t))
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := owner.CancelAccountOrder(f.ctx, commerce.OrderCancellationRequest{OrderNo: p.OrderNo, IdempotencyKey: "native-stock-cancel"})
	if err != nil || cancelled.Status != commerce.OrderCancellationStatusCompleted {
		t.Fatalf("real cancel=%+v / %+v", cancelled, err)
	}
	assertTrafficPackageOccupancy(t, f.accessoryCheckoutFixture, 1)
	reader, err := commerce.NewCancellationEvidenceReader(f.database)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewAccessoryCancellationConsumer(f.database, reader)
	if err != nil {
		t.Fatal(err)
	}
	input := productfulfillment.OrderCancellationInput{OrderNo: p.OrderNo, CancellationNo: cancelled.CancellationNo}
	for i := range 2 {
		res, handled, consumeErr := consumer.ConsumeAddonCancellation(f.ctx, input)
		if consumeErr != nil || !handled || res.Cancelled != 1-i {
			t.Fatalf("cancel delivery=%+v handled=%t / %+v", res, handled, consumeErr)
		}
	}
	assertTrafficPackageOccupancy(t, f.accessoryCheckoutFixture, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='cancelled'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_order_cancellation WHERE status_code='completed'`, 1)
}

func TestNativeCheckoutRejectsStaleAndConcurrentRequestsPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	validator := &nativeCheckoutParameterBoundary{target: f.target, reject: true}
	f.native.pricing.parameters = validator
	if _, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionTrafficAdd, PackageNo: "POL_checkout_test", IdempotencyKey: "native-guard-denied-source"}); err == nil {
		t.Fatal("resource validator rejection ignored")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_quote`, 0)
	validator.reject = false

	quote := func(key string) *servercommercialcap.Quote {
		t.Helper()
		q, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionTrafficAdd, PackageNo: "POL_checkout_test", IdempotencyKey: key})
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	q1, q2 := quote("native-guard-first"), quote("native-guard-second")
	for _, test := range []struct {
		name   string
		mutate func(*servercommercialcap.TargetSnapshot)
	}{
		{"foreign-owner", func(target *servercommercialcap.TargetSnapshot) { target.CustomerNo = "CUS_not_owner" }},
		{"stale-version", func(target *servercommercialcap.TargetSnapshot) { target.EntitlementVersion++ }},
		{"changed-route", func(target *servercommercialcap.TargetSnapshot) { target.PoolNo = "POOL_other" }},
		{"missing-action", func(target *servercommercialcap.TargetSnapshot) { target.Actions = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := f.target
			test.mutate(&target)
			if _, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: target, QuoteNo: q1.QuoteNo, IdempotencyKey: "native-guard-checkout"}); err == nil {
				t.Fatal("invalid target accepted")
			}
		})
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 0)
	f.native.pricing.audit = nativeCheckoutRejectedAudit{}
	if _, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: q1.QuoteNo, IdempotencyKey: "native-guard-audit-failure"}); err == nil {
		t.Fatal("failed audit committed checkout")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_order`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_billing_invoice`, 1)
	assertTrafficPackageOccupancy(t, f.accessoryCheckoutFixture, 0)
	f.native.pricing.audit = trafficTestAudit{}
	if validator.calls != 3 {
		t.Fatalf("resource validation calls=%d", validator.calls)
	}
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for i, q := range []*servercommercialcap.Quote{q1, q2} {
		wait.Add(1)
		go func(i int, q *servercommercialcap.Quote) {
			defer wait.Done()
			key := []string{"native-guard-winner-a", "native-guard-winner-b"}[i]
			_, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: q.QuoteNo, IdempotencyKey: key})
			results <- err
		}(i, q)
	}
	wait.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent checkout successes=%d", success)
	}
	assertTrafficPackageOccupancy(t, f.accessoryCheckoutFixture, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_order`, 2)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE version=1`, 1)
}

// nativeCheckoutParameterBoundary proves local resource identity and policy-frozen
// traffic capacity reach the actual injected validator before durable pricing.
type nativeCheckoutParameterBoundary struct {
	target servercommercialcap.TargetSnapshot
	reject bool
	calls  int
}

func (v *nativeCheckoutParameterBoundary) ValidateParameters(_ context.Context, route serveroperationresourcecap.CapabilityRoute, action serveroperationresourcecap.Action, raw json.RawMessage) error {
	v.calls++
	var value struct {
		TrafficGB int `json:"trafficGb"`
	}
	if v.reject || route.ServiceNo != v.target.LocalResourceNo || route.ChannelNo != v.target.ChannelNo || route.PoolNo != v.target.PoolNo || action != serveroperationresourcecap.ActionTrafficAdd || json.Unmarshal(raw, &value) != nil || value.TrafficGB != 100 {
		return errors.New("resource validator denied invalid local target or parameters")
	}
	return nil
}

type nativeCheckoutRejectedAudit struct{ trafficTestAudit }

func (nativeCheckoutRejectedAudit) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	return errors.New("native checkout audit unavailable")
}
