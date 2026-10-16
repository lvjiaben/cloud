// This file verifies set-based batch tag writes and customer-searchable audit evidence.

package customermanagement

import (
	"bytes"
	"context"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

func TestBatchTagMutationPostgresIsSetBasedAuditedAndReplaySafe(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	current := managementTestBizCtx{current: bizctxcap.CurrentContext{
		UserID: 71, TenantID: 0, DataScope: 2,
	}}
	principalSvc := &managementTestPrincipal{audience: activeStaffAudience()}
	authorization := &managementTestAuthz{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permissionCustomerTagBatch): true,
		authz.PermissionKey("idc:admin:audit:list"):     true,
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
	created := time.Date(2026, 8, 8, 13, 0, 0, 0, time.UTC)
	customers := []string{
		"CUS_44444444444444444444444444444444",
		"CUS_55555555555555555555555555555555",
	}
	for index, customerNo := range customers {
		insertManagementCustomer(
			t,
			ctx,
			customerNo,
			customer.TypeIndividual,
			customer.VerificationUnverified,
			"Batch Customer "+string(rune('A'+index)),
			created.Add(time.Duration(index)*time.Second),
		)
	}
	tagNo := "TAG_44444444444444444444444444444444"
	if _, err = dao.CustomerTag.Ctx(ctx).Data(do.CustomerTag{
		TenantId: 0, TagNo: tagNo, TagCode: "batch-managed",
		NameZhCn: "批量维护", NameEnUs: "Batch Managed", ColorToken: "primary",
		StatusCode: "active", Version: 1,
	}).Insert(); err != nil {
		t.Fatalf("insert batch tag definition: %v", err)
	}
	assign := TagBatchInput{
		Action: TagBatchAssign, CustomerNos: []string{customers[1], customers[0]},
		TagNos: []string{tagNo}, Reason: "运营批量维护", IdempotencyKey: "batch-assign-0001",
	}
	assigned, err := service.BatchUpdateTags(ctx, assign)
	if err != nil || assigned == nil || assigned.Replayed || assigned.CustomerCount != 2 || assigned.TagCount != 1 {
		t.Fatalf("assign batch tags result=%+v err=%v", assigned, err)
	}
	activeCount, err := dao.CustomerTagAssignment.Ctx(ctx).Where(do.CustomerTagAssignment{
		TenantId: 0, TagNo: tagNo, StatusCode: "active",
	}).Count()
	if err != nil || activeCount != 2 {
		t.Fatalf("active tag assignments=%d err=%v", activeCount, err)
	}
	replayed, err := service.BatchUpdateTags(ctx, assign)
	if err != nil || replayed == nil || !replayed.Replayed || replayed.AuditNo != assigned.AuditNo {
		t.Fatalf("replay batch tags result=%+v err=%v", replayed, err)
	}
	activeCount, err = dao.CustomerTagAssignment.Ctx(ctx).Where(do.CustomerTagAssignment{
		TenantId: 0, TagNo: tagNo, StatusCode: "active",
	}).Count()
	if err != nil || activeCount != 2 {
		t.Fatalf("replayed tag assignments=%d err=%v", activeCount, err)
	}
	// New names and existing numbers commit together; retry and another
	// customer using the same name reuse the shared definition.
	named := TagBatchInput{Action: TagBatchAssign, CustomerNos: customers, TagNos: []string{tagNo}, TagNames: []string{"输入回车标签"}, IdempotencyKey: "entered-tag-batch-0001"}
	result, namedErr := service.BatchUpdateTags(ctx, named)
	if namedErr != nil || result.TagCount != 2 {
		t.Fatalf("named batch result=%+v err=%v", result, namedErr)
	}
	if _, namedErr = service.BatchUpdateTags(ctx, named); namedErr != nil {
		t.Fatal(namedErr)
	}
	named.IdempotencyKey = "entered-tag-batch-0002"
	if _, namedErr = service.BatchUpdateTags(ctx, named); namedErr != nil {
		t.Fatal(namedErr)
	}
	namedCount, namedErr := dao.CustomerTag.Ctx(ctx).Where(do.CustomerTag{TenantId: 0, NameZhCn: "输入回车标签"}).Count()
	if namedErr != nil || namedCount != 1 {
		t.Fatalf("shared definition count=%d err=%v", namedCount, namedErr)
	}
	named.CustomerNos = []string{"CUS_ffffffffffffffffffffffffffffffff"}
	named.TagNames = []string{"失败不留标签"}
	named.IdempotencyKey = "entered-tag-batch-0003"
	if _, namedErr = service.BatchUpdateTags(ctx, named); namedErr == nil {
		t.Fatal("missing customer accepted")
	}
	namedCount, namedErr = dao.CustomerTag.Ctx(ctx).Where(do.CustomerTag{TenantId: 0, NameZhCn: "失败不留标签"}).Count()
	if namedErr != nil || namedCount != 0 {
		t.Fatalf("failed definition count=%d err=%v", namedCount, namedErr)
	}
	if _, err = dao.CustomerSubject.Ctx(ctx).
		Where(do.CustomerSubject{TenantId: 0, CustomerNo: customers[0]}).
		Data(do.CustomerSubject{StatusCode: "anonymized"}).Update(); err != nil {
		t.Fatalf("anonymize customer before replay: %v", err)
	}
	if _, err = dao.CustomerTag.Ctx(ctx).
		Where(do.CustomerTag{TenantId: 0, TagNo: tagNo}).
		Data(do.CustomerTag{StatusCode: "archived"}).Update(); err != nil {
		t.Fatalf("archive tag before replay: %v", err)
	}
	replayedAfterStateChange, err := service.BatchUpdateTags(ctx, assign)
	if err != nil || replayedAfterStateChange == nil || !replayedAfterStateChange.Replayed ||
		replayedAfterStateChange.AuditNo != assigned.AuditNo {
		t.Fatalf("replay after state changes result=%+v err=%v", replayedAfterStateChange, err)
	}
	if _, err = dao.CustomerSubject.Ctx(ctx).
		Where(do.CustomerSubject{TenantId: 0, CustomerNo: customers[0]}).
		Data(do.CustomerSubject{StatusCode: "active"}).Update(); err != nil {
		t.Fatalf("restore customer after replay: %v", err)
	}
	if _, err = dao.CustomerTag.Ctx(ctx).
		Where(do.CustomerTag{TenantId: 0, TagNo: tagNo}).
		Data(do.CustomerTag{StatusCode: "active"}).Update(); err != nil {
		t.Fatalf("restore tag after replay: %v", err)
	}
	revoked, err := service.BatchUpdateTags(ctx, TagBatchInput{
		Action: TagBatchRevoke, CustomerNos: customers, TagNos: []string{tagNo},
		Reason: "运营策略调整", IdempotencyKey: "batch-revoke-0001",
	})
	if err != nil || revoked == nil || revoked.Replayed {
		t.Fatalf("revoke batch tags result=%+v err=%v", revoked, err)
	}
	revokedCount, err := dao.CustomerTagAssignment.Ctx(ctx).Where(do.CustomerTagAssignment{
		TenantId: 0, TagNo: tagNo, StatusCode: "revoked",
	}).Count()
	if err != nil || revokedCount != 2 {
		t.Fatalf("revoked tag assignments=%d err=%v", revokedCount, err)
	}
	auditCount, err := dao.BusinessAuditEvent.Ctx(ctx).Count()
	if err != nil || auditCount != 4 {
		t.Fatalf("batch audit events=%d err=%v", auditCount, err)
	}
	linkCount, err := dao.BusinessAuditCustomerLink.Ctx(ctx).Count()
	if err != nil || linkCount != 8 {
		t.Fatalf("batch audit customer links=%d err=%v", linkCount, err)
	}
	page, err := auditService.Search(ctx, audit.SearchInput{CustomerNo: customers[0], Limit: 10})
	if err != nil || len(page.Events) != 4 {
		t.Fatalf("customer batch audit timeline=%+v err=%v", page, err)
	}
}
