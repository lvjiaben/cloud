// This regression verifies bounded keyset rotation over real PostgreSQL candidates, including permanent failures.
package productfulfillment

import (
	"context"
	"errors"
	"os"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

type recoveryOwnerFake struct{ seen []string }

// TestGateRecoveryRejectsMissingRuntimeDependencies ensures a zero worker cannot enter database work.
func TestGateRecoveryRejectsMissingRuntimeDependencies(t *testing.T) {
	var worker *gateRecoveryWorker
	if _, err := worker.Run(context.Background(), 20); err == nil {
		t.Fatal("nil worker accepted")
	}
	if _, err := NewGateRecoveryWorker(nil, &recoveryOwnerFake{}); err == nil {
		t.Fatal("nil database accepted")
	}
	if _, err := NewOwnerGateReleaser(nil, recoveryFrozen{}, nil, recoveryReasons{}); err == nil {
		t.Fatal("missing owner dependencies accepted")
	}
}

func (f *recoveryOwnerFake) ReleaseOwnerGates(_ context.Context, input OwnerGateReleaseInput) (*SettlementResult, error) {
	f.seen = append(f.seen, input.OrderNo)
	if input.OrderNo == "ORD_a" {
		return nil, errors.New("owner unavailable")
	}
	return &SettlementResult{TaskCount: 1}, nil
}

// TestGateRecoveryFairRotationPostgreSQL proves failures and unchanged gates do not monopolize bounded passes.
func TestGateRecoveryFairRotationPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	rollback := errors.New("rollback gate rotation")
	err = db.Transaction(t.Context(), func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent(tenant_id int,order_no text,order_item_no text,strategy_code text,status_code text);
INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES
(0,'ORD_a','OIT_a','payment_confirmed_plugin','gate_pending'),
(0,'ORD_b','OIT_b','payment_confirmed_plugin','gate_pending'),
(0,'ORD_c','OIT_c','payment_confirmed_plugin','gate_pending'),
(0,'ORD_d','OIT_d','payment_confirmed_plugin','canceled'),
(0,'ORD_e','OIT_e','manual','gate_pending'),
(1,'ORD_f','OIT_f','payment_confirmed_plugin','gate_pending');`); err != nil {
			return err
		}
		owner := &recoveryOwnerFake{}
		worker, err := NewGateRecoveryWorker(db, owner)
		if err != nil {
			return err
		}
		for index := 0; index < 4; index++ {
			got, err := worker.Run(ctx, 1)
			if got.Selected != 1 || (index%3 == 0 && (err == nil || got.Failed != 1)) || (index%3 != 0 && (err != nil || got.Queued != 1)) {
				t.Fatalf("pass=%d result=%+v error=%v", index, got, err)
			}
		}
		if len(owner.seen) != 4 || owner.seen[0] != "ORD_a" || owner.seen[1] != "ORD_b" || owner.seen[2] != "ORD_c" || owner.seen[3] != "ORD_a" {
			t.Fatalf("unfair rotation or leaked target: %v", owner.seen)
		}
		// Within one pass the first failure must not prevent the subsequent owner call.
		got, err := worker.Run(ctx, 3)
		if err != nil || got.Selected != 2 || got.Queued != 2 {
			t.Fatalf("later items were starved: %+v %v", got, err)
		}
		got, err = worker.Run(ctx, 3)
		if err == nil || got.Selected != 3 || got.Queued != 2 || got.Failed != 1 {
			t.Fatalf("failed first item ended batch: %+v %v", got, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
