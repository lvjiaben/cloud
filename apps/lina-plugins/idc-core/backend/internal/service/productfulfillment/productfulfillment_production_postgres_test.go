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
		if _, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_commerce_order(tenant_id INT,order_no TEXT,risk_decision_code TEXT);
CREATE TEMP TABLE plugin_idc_core_commerce_order_item(tenant_id INT,order_no TEXT,order_item_no TEXT);
INSERT INTO plugin_idc_core_commerce_order VALUES(0,'ORD_gate','approved');
INSERT INTO plugin_idc_core_commerce_order_item VALUES(0,'ORD_gate','item_gate');`); err != nil {
			return err
		}
		gate, err := NewOwnerGateValidator(db)
		if err != nil {
			return err
		}
		for _, scenario := range []struct {
			risk  string
			ready bool
		}{
			{"approved", true}, {"pending", false}, {"rejected", false},
		} {
			if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_commerce_order SET risk_decision_code=?`, scenario.risk); err != nil {
				return err
			}
			result, err := gate.EvaluateFulfillmentRelease(ctx, "ORD_gate", []string{"item_gate"})
			if err != nil || len(result) != 1 || result["item_gate"] != scenario.ready {
				t.Fatalf("risk=%s gate=%v err=%v", scenario.risk, result, err)
			}
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}

// A paid order may freeze a newer connection than its immutable product version.
// Recovery must reconstruct those two independent facts without losing either.
func TestPaidResourceRecoveryUsesFrozenConnectionAndOriginalSpecificationPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
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
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		_, createErr := db.Exec(ctx, `
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent(tenant_id INT,intent_no TEXT,order_no TEXT,order_item_no TEXT,product_no TEXT,version_no TEXT,customer_no TEXT,configuration_snapshot_no TEXT,interface_instance_no TEXT,interface_configuration_revision INT,unit_sequence INT,quantity INT,entitlement_owner_id TEXT);
CREATE TEMP TABLE plugin_idc_core_commerce_structured_configuration_snapshot(tenant_id INT,snapshot_no TEXT,order_item_no TEXT,order_no TEXT,configuration_revision TEXT,typed_value_snapshot JSONB);
CREATE TEMP TABLE plugin_idc_core_catalog_product_version_configuration(tenant_id INT,version_no TEXT,interface_instance_no TEXT,interface_configuration_revision INT,customer_declaration_snapshot JSONB,merchant_value_snapshot JSONB,merchant_value_digest TEXT,customer_presentation_snapshot JSONB DEFAULT '{}',product_declaration_snapshot JSONB DEFAULT '{}');
CREATE TEMP TABLE plugin_idc_core_product_automation_interface_configuration_revision(tenant_id INT,interface_instance_no TEXT,revision INT,resource_owner_configuration_revision TEXT);
INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES(0,'intent_recovery','order_recovery','item_recovery','product_recovery','version_recovery','customer_recovery','snapshot_recovery','pfi_11111111111111111111111111111111',6,1,1,'idc-resource');
INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot VALUES(0,'snapshot_recovery','item_recovery','order_recovery','config_recovery','{"operating_system":"97"}');
INSERT INTO plugin_idc_core_catalog_product_version_configuration(tenant_id,version_no,interface_instance_no,interface_configuration_revision,customer_declaration_snapshot,merchant_value_snapshot,merchant_value_digest) VALUES(0,'version_recovery','pfi_11111111111111111111111111111111',4,'{}','{"cpu":2,"memory":2000}',repeat('a',64));
INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision VALUES(0,'pfi_11111111111111111111111111111111',4,'rci_44444444444444444444444444444444'),(0,'pfi_11111111111111111111111111111111',6,'rci_66666666666666666666666666666666');`)
		if createErr != nil {
			return createErr
		}
		service := &serviceImpl{database: db}
		request, loadErr := service.loadPaidResourceRequest(ctx, "intent_recovery")
		if loadErr != nil {
			return loadErr
		}
		if request.InterfaceConfigurationRevision != "rci_66666666666666666666666666666666" || string(request.ProductConfiguration) != `{"cpu": 2, "memory": 2000}` || string(request.CustomerConfiguration) != `{"operating_system":"97"}` || request.Fulfillment.Quantity != 1 {
			t.Fatalf("paid recovery facts changed: %+v", request)
		}
		if _, deleteErr := db.Exec(ctx, `DELETE FROM plugin_idc_core_product_automation_interface_configuration_revision WHERE revision=6`); deleteErr != nil {
			return deleteErr
		}
		if _, loadErr = service.loadPaidResourceRequest(ctx, "intent_recovery"); loadErr == nil {
			t.Fatal("missing frozen connection accepted")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
