package serveroperation

import (
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func TestAllowedActionUsesFrozenEntitlementAndRuntimeMatrix(t *testing.T) {
	for _, entitlement := range []string{"active", "grace"} {
		running := &serviceRow{EntitlementStatusCode: entitlement, RuntimeStatusCode: "running"}
		for _, action := range []serveroperationresourcecap.Action{
			serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionReboot,
			serveroperationresourcecap.ActionResetPassword, serveroperationresourcecap.ActionConsole,
			serveroperationresourcecap.ActionReinstall, serveroperationresourcecap.ActionSnapshotCreate,
			serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionBackupCreate,
			serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionSecurityGroupAttach,
			serveroperationresourcecap.ActionSecurityGroupDetach, serveroperationresourcecap.ActionNATCreate,
			serveroperationresourcecap.ActionNATDelete, serveroperationresourcecap.ActionVirtualMediaMount,
			serveroperationresourcecap.ActionVirtualMediaUnmount, serveroperationresourcecap.ActionTrafficAdd,
			serveroperationresourcecap.ActionChangeIP,
		} {
			if !allowedAction(running, action) {
				t.Fatalf("%s running rejected %s", entitlement, action)
			}
		}
		stopped := &serviceRow{EntitlementStatusCode: entitlement, RuntimeStatusCode: "stopped"}
		for _, action := range []serveroperationresourcecap.Action{
			serveroperationresourcecap.ActionStart, serveroperationresourcecap.ActionResetPassword,
			serveroperationresourcecap.ActionReinstall, serveroperationresourcecap.ActionSnapshotCreate,
			serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionSnapshotRestore,
			serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionBackupDelete,
			serveroperationresourcecap.ActionBackupRestore, serveroperationresourcecap.ActionSecurityGroupAttach,
			serveroperationresourcecap.ActionSecurityGroupDetach, serveroperationresourcecap.ActionNATCreate,
			serveroperationresourcecap.ActionNATDelete, serveroperationresourcecap.ActionVirtualMediaMount,
			serveroperationresourcecap.ActionVirtualMediaUnmount, serveroperationresourcecap.ActionTrafficAdd,
			serveroperationresourcecap.ActionChangeIP,
		} {
			if !allowedAction(stopped, action) {
				t.Fatalf("%s stopped rejected %s", entitlement, action)
			}
		}
		if allowedAction(stopped, serveroperationresourcecap.ActionConsole) {
			t.Fatal("stopped server exposed console")
		}
	}
	for _, runtime := range []string{"suspended", "terminating", "unknown", "changing"} {
		if allowedAction(&serviceRow{EntitlementStatusCode: "active", RuntimeStatusCode: runtime},
			serveroperationresourcecap.ActionStart) {
			t.Fatalf("unsafe runtime %s accepted an action", runtime)
		}
	}
	if allowedAction(&serviceRow{EntitlementStatusCode: "active", RuntimeStatusCode: "running"},
		serveroperationresourcecap.ActionSnapshotRestore) {
		t.Fatal("running server accepted snapshot restore")
	}
	if allowedAction(&serviceRow{EntitlementStatusCode: "active", RuntimeStatusCode: "suspended"},
		serveroperationresourcecap.ActionTrafficAdd) {
		t.Fatal("unproven suspended server accepted traffic package")
	}
	if !allowedAction(&serviceRow{EntitlementStatusCode: "active", RuntimeStatusCode: "suspended", TrafficSuspended: true},
		serveroperationresourcecap.ActionTrafficAdd) {
		t.Fatal("traffic-suspended server rejected traffic package")
	}
}

func TestStaffAccessoryPermissionIsExact(t *testing.T) {
	tests := map[serveroperationresourcecap.Action]string{
		serveroperationresourcecap.ActionSnapshotCreate:      PermissionSnapshotCreate,
		serveroperationresourcecap.ActionSnapshotDelete:      PermissionSnapshotDelete,
		serveroperationresourcecap.ActionSnapshotRestore:     PermissionSnapshotRestore,
		serveroperationresourcecap.ActionBackupCreate:        PermissionBackupCreate,
		serveroperationresourcecap.ActionBackupDelete:        PermissionBackupDelete,
		serveroperationresourcecap.ActionBackupRestore:       PermissionBackupRestore,
		serveroperationresourcecap.ActionSecurityGroupAttach: PermissionSecurityGroupAttach,
		serveroperationresourcecap.ActionSecurityGroupDetach: PermissionSecurityGroupDetach,
		serveroperationresourcecap.ActionNATCreate:           PermissionNATCreate,
		serveroperationresourcecap.ActionNATDelete:           PermissionNATDelete,
		serveroperationresourcecap.ActionVirtualMediaMount:   PermissionVirtualMediaMount,
		serveroperationresourcecap.ActionVirtualMediaUnmount: PermissionVirtualMediaUnmount,
		serveroperationresourcecap.ActionTrafficAdd:          PermissionTrafficAdd,
	}
	for action, want := range tests {
		if got := staffAccessoryPermission(action); got != want {
			t.Fatalf("permission for %q=%q, want %q", action, got, want)
		}
	}
	for _, action := range []serveroperationresourcecap.Action{serveroperationresourcecap.ActionStart} {
		if got := staffAccessoryPermission(action); got != "" {
			t.Fatalf("unfrozen staff action %q exposed permission %q", action, got)
		}
	}
}

func TestStaffPricedAccessoryActionsUseQuoteCheckout(t *testing.T) {
	for _, action := range []serveroperationresourcecap.Action{
		serveroperationresourcecap.ActionSnapshotCreate,
		serveroperationresourcecap.ActionBackupCreate,
		serveroperationresourcecap.ActionTrafficAdd,
	} {
		if !isPricedAccessoryCreate(action) {
			t.Fatalf("priced staff action %q bypassed quote checkout", action)
		}
	}
	for _, action := range []serveroperationresourcecap.Action{
		serveroperationresourcecap.ActionSnapshotDelete,
		serveroperationresourcecap.ActionBackupRestore,
		serveroperationresourcecap.ActionNATCreate,
	} {
		if isPricedAccessoryCreate(action) {
			t.Fatalf("free staff action %q was routed through priced checkout", action)
		}
	}
}

func TestCommandRequiresCatalogedStaffPermissionAndNoCustomerPermission(t *testing.T) {
	base := Command{
		CustomerNo: "CUS_1", ServiceNo: "SVC_1", Action: serveroperationresourcecap.ActionSnapshotDelete,
		Parameters: []byte(`{"objectNo":"ACO_1"}`), IdempotencyKey: "request-0001",
		ActorReference: "1001",
	}
	staff := base
	staff.ActorType = "staff"
	staff.RequiredPermission = PermissionSnapshotDelete
	if _, _, err := normalizeCommand(staff); err != nil {
		t.Fatalf("cataloged staff permission rejected: %v", err)
	}
	staff.RequiredPermission = "idc:admin:resource:server-accessory:unknown"
	if _, _, err := normalizeCommand(staff); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("uncataloged staff permission error=%v", err)
	}
	customer := base
	customer.ActorType = "customer"
	customer.RequiredPermission = PermissionSnapshotDelete
	if _, _, err := normalizeCommand(customer); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("customer permission injection error=%v", err)
	}
}

func TestAccessoryActionsUseAddonBusinessOperationType(t *testing.T) {
	if got := operationTypeForAction(serveroperationresourcecap.ActionSnapshotCreate); got != "addon" {
		t.Fatalf("snapshot operation type=%q", got)
	}
	if got := operationTypeForAction(serveroperationresourcecap.ActionStart); got != serveroperationresourcecap.ActionStart {
		t.Fatalf("ordinary operation type=%q", got)
	}
}

func TestTrafficFulfillmentNeverChangesRuntimeState(t *testing.T) {
	for _, original := range []string{"running", "stopped", "suspended"} {
		if got := transitionalRuntime(string(serveroperationresourcecap.ActionTrafficAdd), original); got != original {
			t.Fatalf("traffic dispatch changed runtime from %q to %q", original, got)
		}
		if got := successfulRuntime(string(serveroperationresourcecap.ActionTrafficAdd), original); got != original {
			t.Fatalf("traffic success changed runtime from %q to %q", original, got)
		}
		if got := manualRuntime(string(serveroperationresourcecap.ActionTrafficAdd), original); got != original {
			t.Fatalf("traffic manual review changed runtime from %q to %q", original, got)
		}
	}
}

func TestAccessoryRestoreIsHighRisk(t *testing.T) {
	for _, action := range []serveroperationresourcecap.Action{
		serveroperationresourcecap.ActionSnapshotRestore,
		serveroperationresourcecap.ActionBackupRestore,
	} {
		if !highRiskAction(action, []byte(`{"objectNo":"ACO_1","confirmedServiceNo":"SVC_1"}`)) ||
			operationRiskCode(action, nil) != "high" {
			t.Fatalf("restore action %q is not high risk", action)
		}
	}
}
