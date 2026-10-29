// The queued restore port must work with no browser principal, while refusing
// execution outside the shared transaction and independently rechecking the
// parent and retained object against isolated PostgreSQL state.

package serveroperation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func TestNativeQueuedRestoreTransactionPostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	nativeLifecycleIntent(t, database, "queued-restore", "native-a", "paid", "succeeded")
	created := servercommercialcap.Completion{OperationNo: "queued-restore", EntitlementNo: "native-a", ObjectNo: "paid-restore-object", Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteCreatedAccessory(ctx, database, created)
	}); err != nil {
		t.Fatal(err)
	}
	// Deliberately omit pricing, customer, session and principal dependencies.
	// A queued job cannot borrow or impersonate the original browser session.
	service := &nativeCommercialService{database: database}
	object := servercommercialcap.ObjectRequest{EntitlementNo: "native-a", CustomerNo: "customer", Kind: serveroperationresourcecap.AccessoryKindSnapshot, ObjectNo: created.ObjectNo}
	if err := service.CheckQueuedRestore(ctx, object); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("missing shared transaction accepted: %v", err)
	}
	if err := service.CheckQueuedRestore(nil, object); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil context accepted: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := service.CheckQueuedRestore(canceled, object); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("canceled context accepted: %v", err)
	}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := service.CheckQueuedRestore(ctx, object); err != nil {
			return err
		}
		foreign := object
		foreign.CustomerNo = "other"
		if err := service.CheckQueuedRestore(ctx, foreign); !errors.Is(err, ErrActionUnavailable) {
			t.Fatalf("foreign task customer accepted: %v", err)
		}
		invalid := object
		invalid.Kind = serveroperationresourcecap.AccessoryKindTraffic
		if err := service.CheckQueuedRestore(ctx, invalid); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid object kind accepted: %v", err)
		}
		invalid = object
		invalid.ObjectNo = ""
		if err := service.CheckQueuedRestore(ctx, invalid); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("missing frozen object accepted: %v", err)
		}
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='suspended' WHERE entitlement_no='native-a'`); err != nil {
			return err
		}
		if err := service.CheckQueuedRestore(ctx, object); !errors.Is(err, ErrActionUnavailable) {
			t.Fatalf("current parent suspension bypassed: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
