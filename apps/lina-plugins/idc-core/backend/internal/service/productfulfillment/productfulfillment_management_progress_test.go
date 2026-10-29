// These tests exercise staff-only management progress against exact local
// ownership joins. Temporary tables are dropped at transaction completion.
package productfulfillment

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"os"
	"testing"
)

func TestManagementPreparationPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(context.Background()); closeErr != nil {
			t.Error(closeErr)
		}
	})
	err = database.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, createErr := database.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_entitlement_created_outbox(
 tenant_id INT,entitlement_no TEXT,intent_no TEXT,customer_no TEXT,owner_id TEXT,
 status_code TEXT,attempt_count INT,last_error_code TEXT) ON COMMIT DROP;
 CREATE TEMP TABLE plugin_idc_core_service_entitlement(
 tenant_id INT,entitlement_no TEXT,source_intent_no TEXT,customer_no TEXT,entitlement_owner_id TEXT,status_code TEXT DEFAULT 'active') ON COMMIT DROP;
 INSERT INTO plugin_idc_core_entitlement_created_outbox VALUES(0,'ent-owned','intent-owned','customer-owned','generic-owner','retry_wait',2,'owner_delivery_unavailable');
 INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'ent-owned','intent-owned','customer-owned','generic-owner','active');`); createErr != nil {
			return createErr
		}
		service := &serviceImpl{database: database}
		progress, readErr := service.loadManagementPreparation(ctx, "intent-owned", "customer-owned", "generic-owner")
		if readErr != nil || progress == nil || progress.StatusCode != "preparing" || progress.AttemptCount != 2 || progress.FailureCode != "owner_delivery_unavailable" {
			t.Fatalf("progress=%+v err=%v", progress, readErr)
		}
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code='manual_attention',attempt_count=10,last_error_code='password=PRIVATE'`); updateErr != nil {
			return updateErr
		}
		progress, readErr = service.loadManagementPreparation(ctx, "intent-owned", "customer-owned", "generic-owner")
		if readErr != nil || progress == nil || progress.StatusCode != "needs_attention" || progress.AttemptCount != 10 || progress.FailureCode != "" {
			t.Fatalf("unsafe progress=%+v err=%v", progress, readErr)
		}
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='pending_activation'`); updateErr != nil {
			return updateErr
		}
		progress, readErr = service.loadManagementPreparation(ctx, "intent-owned", "customer-owned", "generic-owner")
		if readErr != nil || progress != nil {
			t.Fatal("pending activation was claimed to be delivered")
		}
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active'`); updateErr != nil {
			return updateErr
		}
		for _, target := range [][3]string{{"another-intent", "customer-owned", "generic-owner"}, {"intent-owned", "another-customer", "generic-owner"}, {"intent-owned", "customer-owned", "another-owner"}} {
			progress, readErr = service.loadManagementPreparation(ctx, target[0], target[1], target[2])
			if readErr != nil || progress != nil {
				t.Fatalf("cross-boundary progress=%+v err=%v", progress, readErr)
			}
		}
		for _, column := range []string{"tenant_id", "entitlement_no", "source_intent_no", "customer_no", "entitlement_owner_id"} {
			statement := "UPDATE plugin_idc_core_service_entitlement SET " + column + "=?"
			invalid, original := any("wrong"), any("")
			switch column {
			case "tenant_id":
				invalid, original = 1, 0
			case "entitlement_no":
				original = "ent-owned"
			case "source_intent_no":
				original = "intent-owned"
			case "customer_no":
				original = "customer-owned"
			case "entitlement_owner_id":
				original = "generic-owner"
			}
			if _, updateErr := database.Exec(ctx, statement, invalid); updateErr != nil {
				return updateErr
			}
			progress, readErr = service.loadManagementPreparation(ctx, "intent-owned", "customer-owned", "generic-owner")
			if readErr != nil || progress != nil {
				t.Fatalf("mismatched %s progress=%+v err=%v", column, progress, readErr)
			}
			if _, updateErr := database.Exec(ctx, statement, original); updateErr != nil {
				return updateErr
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
