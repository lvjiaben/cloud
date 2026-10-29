// This file verifies an administrator recovery can resume a stranded local
// binding round with its old counters retained for audit, without touching a
// live lease, delivered ownership, another customer or an active entitlement.

package productfulfillment

import (
	"context"
	"os"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

func TestResourceProjectionRecoveryIsScopedAndAuditablePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	for _, scenario := range []struct {
		name, state, owner, customer, entitlementState string
		attempts, wantAttempts                         int
		live, wantFact, wantConflict                   bool
	}{
		{name: "old-exhausted-round", state: "manual_attention", owner: provisionresourcecap.OwnerID, customer: "customer-owned", entitlementState: "pending_activation", attempts: 10, wantFact: true},
		{name: "existing-retry-preserves-count", state: "retry_wait", owner: provisionresourcecap.OwnerID, customer: "customer-owned", entitlementState: "pending_activation", attempts: 4, wantAttempts: 4, wantFact: true},
		{name: "pending-is-still-pending", state: "pending", owner: provisionresourcecap.OwnerID, customer: "customer-owned", entitlementState: "pending_activation", wantFact: true},
		{name: "expired-local-lease", state: "delivering", owner: provisionresourcecap.OwnerID, customer: "customer-owned", entitlementState: "pending_activation", attempts: 10, wantFact: true},
		{name: "live-lease-is-not-stolen", state: "delivering", owner: provisionresourcecap.OwnerID, customer: "customer-owned", entitlementState: "pending_activation", attempts: 10, live: true, wantConflict: true},
		{name: "delivered-is-not-rebound", state: "delivered", owner: provisionresourcecap.OwnerID, customer: "customer-owned", entitlementState: "pending_activation", attempts: 2},
		{name: "other-owner-unchanged", state: "manual_attention", owner: "idc-peer", customer: "customer-owned", entitlementState: "pending_activation", attempts: 10},
		{name: "other-customer-unchanged", state: "manual_attention", owner: provisionresourcecap.OwnerID, customer: "customer-other", entitlementState: "pending_activation", attempts: 10},
		{name: "active-service-unchanged", state: "manual_attention", owner: provisionresourcecap.OwnerID, customer: "customer-owned", entitlementState: "active", attempts: 10},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := database.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
				if _, err := database.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_entitlement_created_outbox(tenant_id INT DEFAULT 0,event_no TEXT,intent_no TEXT,entitlement_no TEXT,customer_no TEXT,owner_id TEXT,status_code TEXT,attempt_count INT,last_error_code TEXT,lease_token TEXT,lease_expires_at TIMESTAMPTZ,next_attempt_at TIMESTAMPTZ,updated_at TIMESTAMPTZ) ON COMMIT DROP;
 CREATE TEMP TABLE plugin_idc_core_service_entitlement(tenant_id INT DEFAULT 0,entitlement_no TEXT,source_intent_no TEXT,customer_no TEXT,status_code TEXT) ON COMMIT DROP;`); err != nil {
					return err
				}
				if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_entitlement_created_outbox(event_no,intent_no,entitlement_no,customer_no,owner_id,status_code,attempt_count,last_error_code,lease_token,lease_expires_at,next_attempt_at) VALUES('event-owned','intent-owned','ent-owned',?,?,?,?,'owner_delivery_unavailable',CASE WHEN ? THEN 'active-lease' ELSE '' END,CASE WHEN ? THEN CURRENT_TIMESTAMP+INTERVAL '1 minute' ELSE CURRENT_TIMESTAMP-INTERVAL '1 minute' END,CURRENT_TIMESTAMP+INTERVAL '30 seconds')`, scenario.customer, scenario.owner, scenario.state, scenario.attempts, scenario.live, scenario.live); err != nil {
					return err
				}
				if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,source_intent_no,customer_no,status_code) VALUES('ent-owned','intent-owned',?,?)`, scenario.customer, scenario.entitlementState); err != nil {
					return err
				}
				service := &serviceImpl{database: database}
				current := &IntentDetail{IntentNo: "intent-owned", CustomerNo: "customer-owned"}
				fact, err := service.resumeResourceProjection(ctx, current)
				if scenario.wantConflict {
					if err == nil {
						t.Fatal("live local projection lease was stolen")
					}
					return nil
				}
				if err != nil {
					return err
				}
				if (fact != nil) != scenario.wantFact {
					t.Fatalf("unexpected local projection recovery: %+v", fact)
				}
				row, err := database.GetOne(ctx, `SELECT status_code,attempt_count,last_error_code FROM plugin_idc_core_entitlement_created_outbox`)
				if err != nil {
					return err
				}
				if scenario.wantFact {
					if fact.EventNo != "event-owned" || fact.PreviousAttempts != scenario.attempts || string(fact.PreviousState) != scenario.state || fact.PreviousErrorCode != "owner_delivery_unavailable" || fact.ResumedAttempts != scenario.wantAttempts {
						t.Fatalf("projection history lost before audit: %+v", fact)
					}
					if row["status_code"].String() != "pending" || row["attempt_count"].Int() != scenario.wantAttempts || row["last_error_code"].String() != "" {
						t.Fatalf("local projection round not resumed: %v", row)
					}
				} else if row["status_code"].String() != scenario.state || row["attempt_count"].Int() != scenario.attempts {
					t.Fatalf("unrelated ownership facts changed: %v", row)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
