// Native cancellation tests retain an addon inventory hold until an exact completed owner proof is available.

package serveroperation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type nativeCancellationEvidenceStub struct {
	proof *commerce.CompletedCancellationEvidence
	err   error
}

func (s nativeCancellationEvidenceStub) GetCompletedCancellationEvidence(context.Context, string, string) (*commerce.CompletedCancellationEvidence, error) {
	return s.proof, s.err
}

func TestNativeAccessoryCancellationConcurrentReplayPostgreSQL(t *testing.T) {
	database, _, _ := nativeAccessorySettlementFixture(t, "traffic")
	proof := &commerce.CompletedCancellationEvidence{CancellationNo: "cancel_native", OrderNo: "ord_1", InvoiceNo: "inv_1", QuoteNo: "quo_1", CustomerNo: "cus_1", CurrencyCode: "CNY", CompletedAt: time.Now().UTC()}
	consumer, err := NewAccessoryCancellationConsumer(database, nativeCancellationEvidenceStub{proof: proof})
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	results := make(chan *productfulfillment.OrderCancellationResult, 4)
	failures := make(chan error, 4)
	for range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, handled, consumeErr := consumer.ConsumeAddonCancellation(context.Background(), productfulfillment.OrderCancellationInput{OrderNo: "ord_1", CancellationNo: "cancel_native"})
			if consumeErr != nil {
				failures <- consumeErr
				return
			}
			if !handled {
				failures <- errors.New("native addon fell through to generic purchase")
				return
			}
			results <- result
		}()
	}
	wait.Wait()
	close(results)
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	count := 0
	for result := range results {
		count += result.Cancelled
	}
	if count != 1 {
		t.Fatalf("cancelled projections=%d want=1", count)
	}
	assertNativeAccessoryCounts(t, database, 1, 0)
	occupied, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_type_code='addon' AND status_code<>'cancelled'`)
	if err != nil || occupied.Int() != 0 {
		t.Fatalf("confirmed cancellation retained inventory: count=%v err=%v", occupied, err)
	}
}

func TestNativeAccessoryCancellationKeepsUnprovenInventoryPostgreSQL(t *testing.T) {
	for _, check := range []struct {
		name, sql   string
		wrongProof  bool
		unconfirmed bool
	}{
		{name: "payment closure unknown", unconfirmed: true},
		{name: "paid operation", sql: `UPDATE plugin_idc_core_business_operation SET settlement_no='fst_1',financially_settled_at=CURRENT_TIMESTAMP`},
		{name: "execution started", sql: `UPDATE plugin_idc_core_business_operation SET execution_started_at=CURRENT_TIMESTAMP`},
		{name: "resource result unknown", sql: `UPDATE plugin_idc_core_business_operation SET status_code='manual_review'`},
		{name: "foreign cancellation", wrongProof: true},
		{name: "mismatched native quote", sql: `UPDATE plugin_idc_core_server_accessory_quote SET subject_no='ENT_other'`},
		{name: "event persistence failure", sql: `ALTER TABLE plugin_idc_core_business_operation_event ADD CONSTRAINT reject_native_cancel CHECK(FALSE)`},
	} {
		t.Run(check.name, func(t *testing.T) {
			database, _, _ := nativeAccessorySettlementFixture(t, "traffic")
			proof := &commerce.CompletedCancellationEvidence{CancellationNo: "cancel_native", OrderNo: "ord_1", InvoiceNo: "inv_1", QuoteNo: "quo_1", CustomerNo: "cus_1", CurrencyCode: "CNY", CompletedAt: time.Now().UTC()}
			if check.wrongProof {
				proof.InvoiceNo = "other_invoice"
			}
			reader := nativeCancellationEvidenceStub{proof: proof}
			if check.unconfirmed {
				reader.err = errors.New("payment closure still unknown")
			}
			if check.sql != "" {
				if _, err := database.Exec(t.Context(), check.sql); err != nil {
					t.Fatal(err)
				}
			}
			consumer, err := NewAccessoryCancellationConsumer(database, reader)
			if err != nil {
				t.Fatal(err)
			}
			if _, handled, err := consumer.ConsumeAddonCancellation(t.Context(), productfulfillment.OrderCancellationInput{OrderNo: "ord_1", CancellationNo: "cancel_native"}); err == nil || !handled {
				t.Fatalf("unsafe cancellation accepted or fell through: handled=%v err=%v", handled, err)
			}
			assertNativeAccessoryCounts(t, database, 0, 0)
			occupied, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_type_code='addon' AND status_code<>'cancelled'`)
			if err != nil || occupied.Int() != 1 {
				t.Fatalf("unsafe cancellation released stock: count=%v err=%v", occupied, err)
			}
		})
	}
}
