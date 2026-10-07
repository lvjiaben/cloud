// These tests use real checkout and cancellation owners in a disposable schema.
package serveroperation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"

	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type cancellationCatalog struct{}

func (cancellationCatalog) ResolvePurchaseConfiguration(context.Context, catalog.PurchaseConfigurationInput) (*catalog.PurchaseConfiguration, error) {
	return nil, errors.New("cancellation must not resolve a new purchase")
}

func (cancellationCatalog) RevalidatePurchaseConfigurations(context.Context, []catalog.PurchaseConfigurationReference) ([]catalog.PurchaseConfigurationState, error) {
	return nil, errors.New("cancellation must not revalidate a new purchase")
}

type cancellationPrincipal struct{}

func (cancellationPrincipal) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 101, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "CUS_checkout_test", Version: 1}, nil
}

type cancellationGeneric struct{ calls int }

func (s *cancellationGeneric) ConsumeOrderCancelled(context.Context, productfulfillment.OrderCancellationInput) (*productfulfillment.OrderCancellationResult, error) {
	s.calls++
	return nil, errors.New("accessory reached generic cancellation")
}

// cancellationEvidenceAltered checks the consumer's contract without disabling
// immutable database guards or making forged facts part of production storage.
type cancellationEvidenceAltered struct {
	reader commerce.CancellationEvidenceReader
	change func(*commerce.CompletedCancellationEvidence)
}

func (s cancellationEvidenceAltered) GetCompletedCancellationEvidence(ctx context.Context, orderNo, cancellationNo string) (*commerce.CompletedCancellationEvidence, error) {
	proof, err := s.reader.GetCompletedCancellationEvidence(ctx, orderNo, cancellationNo)
	if err != nil {
		return nil, err
	}
	copy := *proof
	s.change(&copy)
	return &copy, nil
}

func newAccessoryCancellationFixture(t *testing.T) (accessoryCheckoutFixture, *AccessoryCheckout, commerce.Service, productfulfillment.AddonCancellationConsumer) {
	t.Helper()
	fixture := newAccessoryCheckoutFixture(t)
	prepareAccessoryOriginalServer(t, fixture)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve cancellation metadata migration")
	}
	metadataSQL, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "065-server-customer-metadata.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range dialect.SplitSQLStatements(string(metadataSQL)) {
		if _, err = fixture.database.Exec(fixture.ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	fixture.service.customers = accessoryChainCustomer{}
	fixture.service.audit = &accessoryChainAudit{}
	checkout, err := fixture.service.CheckoutCurrentCustomer(fixture.ctx, AccessoryCheckoutInput{QuoteNo: fixture.row.QuoteNo, IdempotencyKey: "accessory-cancel-test"})
	if err != nil {
		t.Fatalf("real addon checkout: %+v", err)
	}
	assertTrafficPackageOccupancy(t, fixture, 1)
	owner, err := commerce.New(fixture.database, cancellationCatalog{}, cancellationPrincipal{}, fixture.settlementService(t))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := commerce.NewCancellationEvidenceReader(fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewAccessoryCancellationConsumer(fixture.database, reader)
	if err != nil {
		t.Fatal(err)
	}
	return fixture, checkout, owner, consumer
}

func assertTrafficPackageOccupancy(t *testing.T, fixture accessoryCheckoutFixture, want int64) {
	t.Helper()
	var rows []struct{ Quantity int64 }
	if err := fixture.service.trafficPackageOccupancy(fixture.ctx, []string{"POL_checkout_test"}).Scan(&rows); err != nil {
		t.Fatalf("traffic package occupancy: %+v", err)
	}
	var got int64
	if len(rows) > 1 {
		t.Fatalf("duplicate traffic package occupancy rows: %+v", rows)
	}
	if len(rows) == 1 {
		got = rows[0].Quantity
	}
	if got != want {
		t.Fatalf("traffic package occupancy=%d, want %d", got, want)
	}
}

func TestAccessoryCancellationCheckoutAndReplayPostgreSQL(t *testing.T) {
	for _, expiry := range []bool{false, true} {
		t.Run(map[bool]string{false: "customer", true: "expiry"}[expiry], func(t *testing.T) {
			fixture, checkout, owner, addon := newAccessoryCancellationFixture(t)
			var cancellationNo string
			if expiry {
				if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_commerce_order SET created_at=CURRENT_TIMESTAMP-INTERVAL '2 hours',payment_due_at=CURRENT_TIMESTAMP-INTERVAL '1 hour' WHERE order_no=?`, checkout.OrderNo); err != nil {
					t.Fatal(err)
				}
				lifecycle, err := commerce.NewOrderLifecycleService(fixture.database)
				if err != nil {
					t.Fatal(err)
				}
				result, err := lifecycle.ExpireDueOrders(fixture.ctx, 10)
				if err != nil || result.Cancelled != 1 {
					t.Fatalf("real expiry=%+v / %+v", result, err)
				}
				value, err := fixture.database.GetValue(fixture.ctx, `SELECT cancellation_no FROM plugin_idc_core_commerce_order_cancellation WHERE order_no=? AND status_code='completed'`, checkout.OrderNo)
				if err != nil {
					t.Fatal(err)
				}
				cancellationNo = value.String()
			} else {
				cancelled, err := owner.CancelAccountOrder(fixture.ctx, commerce.OrderCancellationRequest{OrderNo: checkout.OrderNo, IdempotencyKey: "customer-cancel-addon"})
				if err != nil || cancelled.Status != commerce.OrderCancellationStatusCompleted {
					t.Fatalf("real cancellation=%+v / %+v", cancelled, err)
				}
				cancellationNo = cancelled.CancellationNo
			}
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_type_code='addon' AND status_code!='cancelled'`, 1)
			generic := &cancellationGeneric{}
			router, err := productfulfillment.NewRoutedCancellationConsumer(generic, addon)
			if err != nil {
				t.Fatal(err)
			}
			input := productfulfillment.OrderCancellationInput{OrderNo: checkout.OrderNo, CancellationNo: cancellationNo}
			const workers = 8
			results := make([]*productfulfillment.OrderCancellationResult, workers)
			errorsFound := make([]error, workers)
			var wait sync.WaitGroup
			for index := range workers {
				wait.Add(1)
				go func(index int) {
					defer wait.Done()
					results[index], errorsFound[index] = router.ConsumeOrderCancelled(fixture.ctx, input)
				}(index)
			}
			wait.Wait()
			cancelledCount := 0
			for index, result := range results {
				if errorsFound[index] != nil || result == nil {
					t.Fatalf("delivery %d: %+v", index, errorsFound[index])
				}
				cancelledCount += result.Cancelled
			}
			if cancelledCount != 1 || generic.calls != 0 {
				t.Fatalf("cancelled=%d generic=%d", cancelledCount, generic.calls)
			}
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_type_code='addon' AND status_code!='cancelled'`, 0)
			assertTrafficPackageOccupancy(t, fixture, 0)
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code IN ('awaiting_financial_settlement','financially_settled','manual_review')`, 0)
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation_event WHERE to_status_code='cancelled'`, 1)
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_operation_execution`, 0)
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_billing_ledger_entry`, 0)
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_instance WHERE version=2 AND runtime_status_code='suspended' AND last_business_operation_no IS NOT NULL`, 1)
		})
	}
}

func TestAccessoryCancellationRejectsUnclosedAndPaidPostgreSQL(t *testing.T) {
	for _, state := range []string{"unpaid", "paid", "payment-processing", "unknown-closure"} {
		t.Run(state, func(t *testing.T) {
			fixture, checkout, owner, consumer := newAccessoryCancellationFixture(t)
			cancellationNo := "CAN_not_completed"
			if state == "paid" {
				fixture.payInvoice(t, checkout.InvoiceNo)
			}
			if state == "payment-processing" || state == "unknown-closure" {
				if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_commerce_order SET status_code='payment_processing' WHERE order_no=?`, checkout.OrderNo); err != nil {
					t.Fatal(err)
				}
				cancelled, err := owner.CancelAccountOrder(fixture.ctx, commerce.OrderCancellationRequest{OrderNo: checkout.OrderNo, IdempotencyKey: "pending-closure-test"})
				if err != nil || cancelled.Status != commerce.OrderCancellationStatusPendingPaymentClosure {
					t.Fatalf("closure request=%+v / %+v", cancelled, err)
				}
				cancellationNo = cancelled.CancellationNo
				if state == "unknown-closure" {
					lifecycle, err := commerce.NewOrderLifecycleService(fixture.database)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = lifecycle.FinalizePaymentClosure(fixture.ctx, commerce.PaymentClosureConfirmation{CancellationNo: cancellationNo, Outcome: commerce.PaymentClosureOutcomeUnknown, Reference: "unknown-attempt", EvidenceDigest: strings.Repeat("a", 64)}); err != nil {
						t.Fatalf("unknown closure: %+v", err)
					}
				}
			}
			if _, handled, err := consumer.ConsumeAddonCancellation(fixture.ctx, productfulfillment.OrderCancellationInput{OrderNo: checkout.OrderNo, CancellationNo: cancellationNo}); err == nil || !handled {
				t.Fatalf("unclosed %s accepted handled=%v err=%v", state, handled, err)
			}
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='awaiting_financial_settlement'`, 1)
			fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation_event WHERE to_status_code='cancelled'`, 0)
		})
	}
}

func TestAccessoryCancellationRejectsWrongFactsAndRollsBackPostgreSQL(t *testing.T) {
	fixture, checkout, owner, consumer := newAccessoryCancellationFixture(t)
	cancelled, err := owner.CancelAccountOrder(fixture.ctx, commerce.OrderCancellationRequest{OrderNo: checkout.OrderNo, IdempotencyKey: "cancel-for-guards"})
	if err != nil {
		t.Fatal(err)
	}
	input := productfulfillment.OrderCancellationInput{OrderNo: checkout.OrderNo, CancellationNo: cancelled.CancellationNo}
	reader, err := commerce.NewCancellationEvidenceReader(fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	for _, mismatch := range []struct {
		name   string
		change func(*commerce.CompletedCancellationEvidence)
	}{
		{"owner-customer", func(proof *commerce.CompletedCancellationEvidence) { proof.CustomerNo = "CUS_other" }},
		{"owner-invoice", func(proof *commerce.CompletedCancellationEvidence) { proof.InvoiceNo = "INV_other" }},
		{"owner-quote", func(proof *commerce.CompletedCancellationEvidence) { proof.QuoteNo = "QUO_other" }},
		{"owner-order", func(proof *commerce.CompletedCancellationEvidence) { proof.OrderNo = "ORD_other" }},
		{"owner-cancellation", func(proof *commerce.CompletedCancellationEvidence) { proof.CancellationNo = "CAN_other" }},
	} {
		t.Run(mismatch.name, func(t *testing.T) {
			altered, err := NewAccessoryCancellationConsumer(fixture.database, cancellationEvidenceAltered{reader: reader, change: mismatch.change})
			if err != nil {
				t.Fatal(err)
			}
			if _, handled, err := altered.ConsumeAddonCancellation(fixture.ctx, input); err == nil || !handled {
				t.Fatal("mismatched owner proof accepted")
			}
		})
	}
	if _, _, err = consumer.ConsumeAddonCancellation(fixture.ctx, productfulfillment.OrderCancellationInput{OrderNo: "ORD_other", CancellationNo: cancelled.CancellationNo}); err == nil {
		t.Fatal("cross-order fact accepted")
	}
	if _, _, err = consumer.ConsumeAddonCancellation(fixture.ctx, productfulfillment.OrderCancellationInput{OrderNo: checkout.OrderNo, CancellationNo: "CAN_other"}); err == nil {
		t.Fatal("wrong cancellation accepted")
	}
	// Corrupt one local relation at a time inside a rolled-back test transaction.
	// No customer account, invoice or operation outside this fixture is touched.
	if _, err = fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code) VALUES ('CUS_cancellation_other','individual','active')`); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ name, query, databaseRejection string }{
		{"cross-customer-cancellation", `UPDATE plugin_idc_core_commerce_order_cancellation SET customer_no='CUS_cancellation_other'`, "order cancellation identity is immutable"},
		{"cross-customer-operation", `UPDATE plugin_idc_core_business_operation SET customer_no='CUS_cancellation_other'`, "business operation intent is immutable"},
		{"missing-original-quote", `UPDATE plugin_idc_core_business_operation SET quote_no=NULL`, "business operation intent is immutable"},
		{"financially-settled-operation", `UPDATE plugin_idc_core_business_operation SET financially_settled_at=CURRENT_TIMESTAMP`, ""},
		{"unknown-manual-state", `UPDATE plugin_idc_core_business_operation SET status_code='manual_review'`, ""},
		{"forged-cancelled-replay", `UPDATE plugin_idc_core_business_operation SET status_code='cancelled',completed_at=CURRENT_TIMESTAMP,version=2`, ""},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			rollback := errors.New("restore corrupt test fact")
			err := fixture.database.Transaction(fixture.ctx, func(ctx context.Context, _ gdb.TX) error {
				if _, err := fixture.database.Exec(ctx, mutation.query); err != nil {
					if mutation.databaseRejection != "" && strings.Contains(err.Error(), mutation.databaseRejection) {
						return rollback
					}
					return err
				}
				if mutation.databaseRejection != "" {
					return errors.New("immutable identity guard did not reject mutation")
				}
				if _, handled, err := consumer.ConsumeAddonCancellation(ctx, input); err == nil || !handled {
					return errors.New("mismatched accessory cancellation accepted")
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatalf("mutation guard: %+v", err)
			}
		})
	}
	// Simulate a later local write failure without changing any business fact.
	if _, err = fixture.database.Exec(fixture.ctx, `ALTER TABLE plugin_idc_core_business_operation_event ADD CONSTRAINT cancellation_event_test_failure CHECK(to_status_code<>'cancelled')`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = consumer.ConsumeAddonCancellation(fixture.ctx, input); err == nil {
		t.Fatal("event insertion failure ignored")
	}
	fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='awaiting_financial_settlement' AND version=1`, 1)
	if _, err = fixture.database.Exec(fixture.ctx, `ALTER TABLE plugin_idc_core_business_operation_event DROP CONSTRAINT cancellation_event_test_failure`); err != nil {
		t.Fatal(err)
	}
	// A harmless metadata revision must not strand a fully cancelled order's stock.
	if _, err = fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_service_instance SET customer_remark='updated while awaiting payment',version=version+1`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = consumer.ConsumeAddonCancellation(fixture.ctx, input); err != nil {
		t.Fatalf("retry after rollback: %+v", err)
	}
	fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_instance WHERE version=3 AND customer_remark='updated while awaiting payment' AND runtime_status_code='suspended'`, 1)
	fixture.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code!='cancelled'`, 0)
}
