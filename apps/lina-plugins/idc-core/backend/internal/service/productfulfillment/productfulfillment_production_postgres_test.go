// This file verifies the production owner gate reads the actual risk column
// and keeps risk and contract requirements independent under real ORM mapping.
package productfulfillment

import (
	"context"
	"errors"
	"os"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestProductionOwnerGateRiskProjectionPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	rollback := errors.New("rollback test fixture")
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_commerce_order(tenant_id INT,order_no TEXT,risk_decision_code TEXT,requires_contract BOOLEAN);
CREATE TEMP TABLE plugin_idc_core_commerce_order_item(tenant_id INT,order_no TEXT,order_item_no TEXT);
INSERT INTO plugin_idc_core_commerce_order VALUES(0,'ORD_gate','approved',false);
INSERT INTO plugin_idc_core_commerce_order_item VALUES(0,'ORD_gate','item_gate');`); err != nil {
			return err
		}
		gate, err := NewOwnerGateValidator(db, nil)
		if err != nil {
			return err
		}
		for _, scenario := range []struct {
			risk            string
			contract, ready bool
		}{
			{"approved", false, true}, {"pending", false, false}, {"rejected", false, false}, {"approved", true, false},
		} {
			if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_commerce_order SET risk_decision_code=?,requires_contract=?`, scenario.risk, scenario.contract); err != nil {
				return err
			}
			result, err := gate.EvaluateFulfillmentRelease(ctx, "ORD_gate", []string{"item_gate"})
			if err != nil || len(result) != 1 || result["item_gate"] != scenario.ready {
				t.Fatalf("risk=%s contract=%v gate=%v err=%v", scenario.risk, scenario.contract, result, err)
			}
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
