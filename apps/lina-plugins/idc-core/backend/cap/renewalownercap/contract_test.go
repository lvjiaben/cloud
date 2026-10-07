package renewalownercap

import (
	"strings"
	"testing"
	"time"
)

func TestTaskRequestRequiresFrozenOwnerAndAdvancingExpiry(t *testing.T) {
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	request := TaskRequest{
		Version: ContractVersion, OwnerID: "idc-resource", OperationNo: "ROP_1",
		EntitlementNo: "ENT_1", PriceSnapshotNo: "RPS_1", PluginID: "server-connector",
		PluginReleaseVersion: "1.0.0", ContractVersion: "v1",
		DeclarationDigest: strings.Repeat("a", 64), RenewalMethod: "renew",
		ExpectedEntitlementVersion: 2, OriginalExpiresAt: now, ProjectedExpiresAt: now.AddDate(0, 1, 0),
	}
	if err := ValidateTaskRequest(request); err != nil {
		t.Fatalf("valid generic renewal request rejected: %v", err)
	}
	request.OwnerID = ""
	if err := ValidateTaskRequest(request); err != ErrTaskConflict {
		t.Fatalf("missing frozen owner returned %v", err)
	}
	request.OwnerID = "idc-resource"
	request.ProjectedExpiresAt = request.OriginalExpiresAt
	if err := ValidateTaskRequest(request); err != ErrTaskConflict {
		t.Fatalf("non-advancing renewal returned %v", err)
	}
}
