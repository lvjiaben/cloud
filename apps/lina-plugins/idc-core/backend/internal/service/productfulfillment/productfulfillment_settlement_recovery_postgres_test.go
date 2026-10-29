// This file verifies recovery selection, bounded rotation and suppression of existing/unknown operations.
package productfulfillment

import (
	"context"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"os"
	"testing"
)

type missingSettlementConsumer struct {
	database gdb.DB
	calls    []string
}

func (f *missingSettlementConsumer) ConsumeFinanciallySettled(ctx context.Context, input SettlementInput) (*SettlementResult, error) {
	f.calls = append(f.calls, input.OrderNo)
	if input.OrderNo == "ORD_fail" {
		return nil, errors.New("still unavailable")
	}
	if _, err := f.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_intent(tenant_id,order_no) VALUES(0,?)`, input.OrderNo); err != nil {
		return nil, err
	}
	return &SettlementResult{OrderNo: input.OrderNo, TaskCount: 1}, nil
}
func TestMissingSettlementRecoveryRotatesAndNeverReplaysUnknown(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(context.Background()); closeErr != nil {
			t.Error(closeErr)
		}
	})
	rollback := errors.New("rollback recovery fixture")
	err = db.Transaction(t.Context(), func(ctx context.Context, tx gdb.TX) error {
		if _, e := tx.Exec(`CREATE TEMP TABLE plugin_idc_core_outbox_event(tenant_id int,event_no text,event_type text,aggregate_no text,correlation_no text,status_code text,last_error_code text);
 CREATE TEMP TABLE plugin_idc_core_financial_settlement(tenant_id int,settlement_no text,order_no text);
 CREATE TEMP TABLE plugin_idc_core_commerce_order(tenant_id int,order_no text,quote_no text,status_code text);
 CREATE TEMP TABLE plugin_idc_core_commerce_quote_snapshot(tenant_id int,quote_no text,purpose_code text);
 CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent(tenant_id int,order_no text);
 INSERT INTO plugin_idc_core_commerce_order VALUES(0,'ORD_fail','Q_fail','paid'),(0,'ORD_good','Q_good','paid'),(0,'ORD_unknown','Q_unknown','paid'),(0,'ORD_existing','Q_existing','paid'),(0,'ORD_addon','Q_addon','paid'),(9,'ORD_foreign','Q_foreign','paid');
 INSERT INTO plugin_idc_core_financial_settlement SELECT tenant_id,'SET_'||order_no,order_no FROM plugin_idc_core_commerce_order;
 INSERT INTO plugin_idc_core_commerce_quote_snapshot SELECT tenant_id,quote_no,CASE WHEN order_no='ORD_addon' THEN 'addon' ELSE 'new_purchase' END FROM plugin_idc_core_commerce_order;
 INSERT INTO plugin_idc_core_outbox_event SELECT tenant_id,CASE WHEN order_no='ORD_fail' THEN 'EV_a' ELSE 'EV_'||order_no END,'commerce.order.financially_settled',order_no,'SET_'||order_no,CASE WHEN order_no='ORD_unknown' THEN 'result_unknown' ELSE 'dead_letter' END,'financial_settlement_delivery_unavailable' FROM plugin_idc_core_commerce_order;
 INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES(0,'ORD_existing');`); e != nil {
			return e
		}
		consumer := &missingSettlementConsumer{database: db}
		worker := &gateRecoveryWorker{database: db, settlements: consumer}
		first, e := worker.recoverSettlements(ctx, 1)
		if e != nil || first.Queued != 1 || first.Selected != 1 {
			t.Fatalf("first bounded recovery: %+v %v", first, e)
		}
		second, e := worker.recoverSettlements(ctx, 1)
		if e == nil || second.Failed != 1 {
			t.Fatalf("failed recovery must remain visible: %+v %v", second, e)
		}
		third, e := worker.recoverSettlements(ctx, 1)
		if e == nil || third.Failed != 1 {
			t.Fatalf("rotation should revisit only unresolved local delivery: %+v %v", third, e)
		}
		for _, no := range consumer.calls {
			if no != "ORD_good" && no != "ORD_fail" {
				t.Fatalf("unsafe selection %s", no)
			}
		}
		count, e := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE order_no='ORD_good'`)
		if e != nil || count.Int() != 1 {
			t.Fatal("duplicate recovered intent", e)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
