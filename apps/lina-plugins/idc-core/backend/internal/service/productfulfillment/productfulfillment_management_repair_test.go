// These tests isolate repair eligibility from provider code and enforce the
// same bounded ownership predicate on every lifecycle and queue boundary.
package productfulfillment

import (
	"context"
	"os"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type deniedManagementRepairAuthorizer struct{ ManagementAuthorizer }

func (deniedManagementRepairAuthorizer) AuthorizeRecovery(context.Context, string) (Authorization, error) {
	return Authorization{}, context.Canceled
}

type unreachableManagementRepairAudit struct{ idccap.BusinessAuditWriter }

func TestManagementRepairAuthorizationBeforeDatabase(t *testing.T) {
	service := &serviceImpl{authorizer: deniedManagementRepairAuthorizer{}, audit: unreachableManagementRepairAudit{}}
	if _, err := service.RepairManagement(t.Context(), ManagementRepairInput{IntentNo: "intent-owned"}); err != context.Canceled {
		t.Fatalf("unauthorized command reached database: %v", err)
	}
}

func TestManagementRepairCandidatePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := db.Close(context.Background()); e != nil {
			t.Error(e)
		}
	})
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, e := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_entitlement_created_outbox(tenant_id INT,event_no TEXT,entitlement_no TEXT,intent_no TEXT,customer_no TEXT,owner_id TEXT,status_code TEXT,attempt_count INT,last_error_code TEXT,lease_token TEXT,lease_expires_at TIMESTAMPTZ,owner_reference TEXT,receipt_evidence_digest TEXT) ON COMMIT DROP;
  CREATE TEMP TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no TEXT,source_intent_no TEXT,customer_no TEXT,entitlement_owner_id TEXT,status_code TEXT,order_item_no TEXT) ON COMMIT DROP;
  CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent(tenant_id INT,intent_no TEXT,customer_no TEXT,entitlement_owner_id TEXT,order_no TEXT,order_item_no TEXT,status_code TEXT,financially_settled_at TIMESTAMPTZ,settlement_no TEXT) ON COMMIT DROP;
  CREATE TEMP TABLE plugin_idc_core_refund_request_item(tenant_id INT,refund_no TEXT,order_item_no TEXT) ON COMMIT DROP;
  CREATE TEMP TABLE plugin_idc_core_refund_request(tenant_id INT,refund_no TEXT,status_code TEXT) ON COMMIT DROP;
  INSERT INTO plugin_idc_core_entitlement_created_outbox VALUES(0,'event','ent','intent','customer','idc-resource','manual_attention',10,'owner_delivery_unavailable','',NULL,'','');
  INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'ent','intent','customer','idc-resource','active','item');
  INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES(0,'intent','customer','idc-resource','order','item','completed',CURRENT_TIMESTAMP,'settlement');
  INSERT INTO plugin_idc_core_refund_request_item VALUES(0,'refund','item');
  INSERT INTO plugin_idc_core_refund_request VALUES(0,'refund','rejected');`); e != nil {
			return e
		}
		service := &serviceImpl{database: db}
		current := &IntentDetail{IntentNo: "intent", CustomerNo: "customer", OrderNo: "order", OrderItemNo: "item"}
		assertCandidate := func(want bool) {
			t.Helper()
			candidate, e := service.managementRepairCandidate(ctx, current, true)
			if e != nil || (candidate != nil) != want {
				t.Fatalf("candidate=%+v want=%v err=%v", candidate, want, e)
			}
		}
		assertCandidate(true)
		for _, fixture := range []struct {
			sql               string
			invalid, original any
		}{
			{"UPDATE plugin_idc_core_product_fulfillment_intent SET status_code=?", "incomplete", "completed"},
			{"UPDATE plugin_idc_core_product_fulfillment_intent SET financially_settled_at=?", nil, "2026-10-22T00:00:00Z"},
			{"UPDATE plugin_idc_core_product_fulfillment_intent SET entitlement_owner_id=?", "generic-owner", "idc-resource"},
			{"UPDATE plugin_idc_core_service_entitlement SET customer_no=?", "other", "customer"},
			{"UPDATE plugin_idc_core_service_entitlement SET order_item_no=?", "other", "item"},
			{"UPDATE plugin_idc_core_service_entitlement SET source_intent_no=?", "other", "intent"},
			{"UPDATE plugin_idc_core_service_entitlement SET status_code=?", "pending_activation", "active"},
			{"UPDATE plugin_idc_core_service_entitlement SET status_code=?", "terminated", "active"},
			{"UPDATE plugin_idc_core_entitlement_created_outbox SET status_code=?", "retry_wait", "manual_attention"},
			{"UPDATE plugin_idc_core_entitlement_created_outbox SET lease_token=?", "busy", ""},
			{"UPDATE plugin_idc_core_entitlement_created_outbox SET lease_expires_at=?", "2026-10-22T00:00:00Z", nil},
			{"UPDATE plugin_idc_core_entitlement_created_outbox SET owner_reference=?", "confirmed", ""},
			{"UPDATE plugin_idc_core_entitlement_created_outbox SET receipt_evidence_digest=?", "confirmed", ""},
			{"UPDATE plugin_idc_core_refund_request SET status_code=?", "pending_review", "rejected"},
			{"UPDATE plugin_idc_core_refund_request SET status_code=?", "completed", "rejected"},
		} {
			if _, e := db.Exec(ctx, fixture.sql, fixture.invalid); e != nil {
				return e
			}
			assertCandidate(false)
			if _, e := db.Exec(ctx, fixture.sql, fixture.original); e != nil {
				return e
			}
			assertCandidate(true)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
