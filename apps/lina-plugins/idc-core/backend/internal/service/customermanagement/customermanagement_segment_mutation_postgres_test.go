// Segment lifecycle integration tests use isolated schemas and the real sealed audit owner.

package customermanagement

import (
	"bytes"
	"context"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"testing"
	"time"
)

func TestCustomerSegmentLifecyclePostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	current := managementTestBizCtx{current: bizctxcap.CurrentContext{
		UserID: 71, TenantID: 0, DataScope: 2,
	}}
	principalSvc := &managementTestPrincipal{audience: activeStaffAudience()}
	authorization := &managementTestAuthz{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permissionCustomerUpdate): true,
		authz.PermissionKey("idc:admin:audit:list"):   true,
	}}
	auditService, err := audit.New(
		current,
		principalSvc,
		authorization,
		audit.Config{
			ReservationLookupKey:  audit.IntegrityKey{ID: "lookup.v1", Secret: bytes.Repeat([]byte{1}, 32)},
			ActiveReplayDigestKey: audit.IntegrityKey{ID: "replay.v1", Secret: bytes.Repeat([]byte{2}, 32)},
			ActiveIntegrityKey:    audit.IntegrityKey{ID: "integrity.v1", Secret: bytes.Repeat([]byte{3}, 32)},
			MaximumSnapshotBytes:  32 * 1024,
			MaximumSnapshotDepth:  8,
			MaximumSnapshotNodes:  512,
			MaximumPageSize:       100,
			MaximumResponseBytes:  512 * 1024,
		},
	)
	if err != nil {
		t.Fatalf("create batch tag audit service: %v", err)
	}
	auditWriter, err := audit.NewBusinessWriterAdapter(auditService)
	if err != nil {
		t.Fatalf("create batch tag audit adapter: %v", err)
	}
	service, err := NewMutation(current, principalSvc, authorization, auditWriter)
	if err != nil {
		t.Fatalf("create batch tag mutation service: %v", err)
	}

	customerNo := "CUS_44444444444444444444444444444444"
	insertManagementCustomer(t, ctx, customerNo, customer.TypeIndividual, customer.VerificationUnverified, "Segment Customer", time.Now())
	segments := service.(SegmentMutationService)
	for _, kind := range []SegmentKind{SegmentKindGroup, SegmentKindTag} {
		input := SegmentMutationInput{CustomerNo: customerNo, Kind: kind, Name: "Customer group", IdempotencyKey: "segment-create-test-" + string(kind)}
		number, err := segments.AddSegment(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := segments.AddSegment(ctx, input)
		if err != nil || replay != number {
			t.Fatalf("replay %s %v", replay, err)
		}
		changed := input
		changed.Name = "Changed name"
		if _, err = segments.AddSegment(ctx, changed); err == nil {
			t.Fatal("changed retry payload accepted")
		}
		remove := SegmentMutationInput{CustomerNo: customerNo, Kind: kind, SegmentNo: number, IdempotencyKey: "segment-remove-test-" + string(kind)}
		if err = segments.RemoveSegment(ctx, remove); err != nil {
			t.Fatal(err)
		}
		if err = segments.RemoveSegment(ctx, remove); err != nil {
			t.Fatal(err)
		}
		var count int
		if kind == SegmentKindGroup {
			count, err = dao.CustomerGroupAssignment.Ctx(ctx).Where(do.CustomerGroupAssignment{CustomerNo: customerNo, GroupNo: number, StatusCode: segmentActive}).Count()
		} else {
			count, err = dao.CustomerTagAssignment.Ctx(ctx).Where(do.CustomerTagAssignment{CustomerNo: customerNo, TagNo: number, StatusCode: segmentActive}).Count()
		}
		if err != nil || count != 0 {
			t.Fatalf("revoked assignment count %d err %v", count, err)
		}
		input.Name = ""
		input.SegmentNo = number
		input.IdempotencyKey = "segment-reassign-test-" + string(kind)
		if _, err = segments.AddSegment(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	missing := SegmentMutationInput{CustomerNo: "CUS_ffffffffffffffffffffffffffffffff", Kind: SegmentKindGroup, Name: "Missing", IdempotencyKey: "segment-missing-test"}
	if _, err := segments.AddSegment(ctx, missing); err == nil {
		t.Fatal("missing customer accepted")
	}
}
