package customermanagement

import (
	"bytes"
	"context"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

func TestCustomerActivityMutationsPreserveHistoryAndRequireBlacklistApprovalPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	current := managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}}
	principalSvc := &managementTestPrincipal{audience: activeStaffAudience()}
	authorization := &managementTestAuthz{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permissionCustomerUpdate): true,
		authz.PermissionKey(permissionCustomerQuery):  true,
	}}
	auditService, err := audit.New(current, principalSvc, authorization, audit.Config{
		ReservationLookupKey:  audit.IntegrityKey{ID: "lookup.v1", Secret: bytes.Repeat([]byte{11}, 32)},
		ActiveReplayDigestKey: audit.IntegrityKey{ID: "replay.v1", Secret: bytes.Repeat([]byte{12}, 32)},
		ActiveIntegrityKey:    audit.IntegrityKey{ID: "integrity.v1", Secret: bytes.Repeat([]byte{13}, 32)},
		MaximumSnapshotBytes:  32 * 1024, MaximumSnapshotDepth: 8, MaximumSnapshotNodes: 512,
		MaximumPageSize: 100, MaximumResponseBytes: 512 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	auditWriter, err := audit.NewBusinessWriterAdapter(auditService)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewMutation(current, principalSvc, authorization, auditWriter)
	if err != nil {
		t.Fatal(err)
	}
	customerNo := "CUS_66666666666666666666666666666666"
	insertManagementCustomer(t, ctx, customerNo, customer.TypeIndividual,
		customer.VerificationUnverified, "Activity Customer", time.Now().UTC())

	first, err := service.AddNote(ctx, NoteInput{CustomerNo: customerNo, Category: "general", Content: "First note", IdempotencyKey: "activity-note-first-0001"})
	if err != nil {
		t.Fatal(err)
	}
	replayedFirst, err := service.AddNote(ctx, NoteInput{CustomerNo: customerNo, Category: "general", Content: "First note", IdempotencyKey: "activity-note-first-0001"})
	if err != nil || replayedFirst.NoteNo != first.NoteNo {
		t.Fatalf("replayed note=%+v err=%v", replayedFirst, err)
	}
	if _, err = service.AddNote(ctx, NoteInput{CustomerNo: customerNo, Category: "general", Content: "Changed meaning", IdempotencyKey: "activity-note-first-0001"}); !bizerr.Is(err, customer.CodeIdempotencyConflict) {
		t.Fatalf("changed note replay error=%v", err)
	}
	second, err := service.AddNote(ctx, NoteInput{CustomerNo: customerNo, Category: "billing", Content: "Corrected note", SupersedesNo: first.NoteNo, IdempotencyKey: "activity-note-second-0001"})
	if err != nil || second.SupersedesNo != first.NoteNo {
		t.Fatalf("corrected note=%+v err=%v", second, err)
	}
	withdrawn, err := dao.CustomerNote.Ctx(ctx).Where(do.CustomerNote{NoteNo: first.NoteNo}).Where("status_code", "withdrawn").Count()
	if err != nil || withdrawn != 1 {
		t.Fatalf("withdrawn note count=%d err=%v", withdrawn, err)
	}

	planned, err := service.PlanFollowUp(ctx, FollowUpPlanInput{CustomerNo: customerNo, Channel: "phone",
		Subject: "Confirm service requirement", PlannedAt: time.Now().UTC().Add(time.Hour), IdempotencyKey: "activity-follow-plan-0001"})
	if err != nil || planned.Status != "planned" {
		t.Fatalf("planned=%+v err=%v", planned, err)
	}
	replayedPlan, err := service.PlanFollowUp(ctx, FollowUpPlanInput{CustomerNo: customerNo, Channel: "phone",
		Subject: "Confirm service requirement", PlannedAt: planned.PlannedAt, IdempotencyKey: "activity-follow-plan-0001"})
	if err != nil || replayedPlan.FollowUpNo != planned.FollowUpNo {
		t.Fatalf("replayed follow-up plan=%+v err=%v", replayedPlan, err)
	}
	completed, err := service.TransitionFollowUp(ctx, FollowUpTransitionInput{CustomerNo: customerNo,
		FollowUpNo: planned.FollowUpNo, Action: "complete", Outcome: "reached", IdempotencyKey: "activity-follow-complete-0001"})
	if err != nil || completed.Status != "completed" {
		t.Fatalf("completed=%+v err=%v", completed, err)
	}
	replayedCompleted, err := service.TransitionFollowUp(ctx, FollowUpTransitionInput{CustomerNo: customerNo,
		FollowUpNo: planned.FollowUpNo, Action: "complete", Outcome: "reached", IdempotencyKey: "activity-follow-complete-0001"})
	if err != nil || replayedCompleted.Status != "completed" {
		t.Fatalf("replayed completed follow-up=%+v err=%v", replayedCompleted, err)
	}

	entry, err := service.CreateBlacklist(ctx, BlacklistCreateInput{CustomerNo: customerNo,
		Scope: "purchase", ReasonCode: "manual_risk_review", RiskLevel: "high", IdempotencyKey: "activity-blacklist-create-0001"})
	if err != nil || entry.Status != "pending" {
		t.Fatalf("blacklist proposal=%+v err=%v", entry, err)
	}
	replayedEntry, err := service.CreateBlacklist(ctx, BlacklistCreateInput{CustomerNo: customerNo,
		Scope: "purchase", ReasonCode: "manual_risk_review", RiskLevel: "high", IdempotencyKey: "activity-blacklist-create-0001"})
	if err != nil || replayedEntry.EntryNo != entry.EntryNo {
		t.Fatalf("replayed blacklist proposal=%+v err=%v", replayedEntry, err)
	}
	activeBefore, _ := dao.CustomerBlacklistEntry.Ctx(ctx).Where(do.CustomerBlacklistEntry{EntryNo: entry.EntryNo, StatusCode: "active"}).Count()
	if activeBefore != 0 {
		t.Fatal("pending blacklist must not take effect")
	}
	approved, err := service.DecideBlacklist(ctx, BlacklistDecisionInput{CustomerNo: customerNo, EntryNo: entry.EntryNo, Action: "approve", IdempotencyKey: "activity-blacklist-approve-0001"})
	if err != nil || approved.Status != "active" {
		t.Fatalf("approved=%+v err=%v", approved, err)
	}
	replayedApproved, err := service.DecideBlacklist(ctx, BlacklistDecisionInput{CustomerNo: customerNo, EntryNo: entry.EntryNo, Action: "approve", IdempotencyKey: "activity-blacklist-approve-0001"})
	if err != nil || replayedApproved.Status != "active" {
		t.Fatalf("replayed blacklist approval=%+v err=%v", replayedApproved, err)
	}
	revoked, err := service.DecideBlacklist(ctx, BlacklistDecisionInput{CustomerNo: customerNo, EntryNo: entry.EntryNo, Action: "revoke", IdempotencyKey: "activity-blacklist-revoke-0001"})
	if err != nil || revoked.Status != "revoked" {
		t.Fatalf("revoked=%+v err=%v", revoked, err)
	}
	replayedRevoked, err := service.DecideBlacklist(ctx, BlacklistDecisionInput{CustomerNo: customerNo, EntryNo: entry.EntryNo, Action: "revoke", IdempotencyKey: "activity-blacklist-revoke-0001"})
	if err != nil || replayedRevoked.Status != "revoked" {
		t.Fatalf("replayed blacklist revocation=%+v err=%v", replayedRevoked, err)
	}
	auditCount, err := dao.BusinessAuditEvent.Ctx(ctx).Count()
	if err != nil || auditCount != 7 {
		t.Fatalf("activity audit count=%d err=%v", auditCount, err)
	}
	reader, err := New(current, principalSvc, authorization, Config{MaximumPageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	notes, err := reader.SearchActivities(ctx, ActivitySearchInput{CustomerNo: customerNo, Kind: ActivityKindNote, Limit: 1})
	if err != nil || len(notes.Items) != 1 || notes.NextCursor == "" || notes.Items[0].Title != "Corrected note" {
		t.Fatalf("note activity page=%+v err=%v", notes, err)
	}
	nextNotes, err := reader.SearchActivities(ctx, ActivitySearchInput{CustomerNo: customerNo, Kind: ActivityKindNote, Limit: 1, Cursor: notes.NextCursor})
	if err != nil || len(nextNotes.Items) != 1 || nextNotes.NextCursor != "" || nextNotes.Items[0].Status != "withdrawn" {
		t.Fatalf("next note activity page=%+v err=%v", nextNotes, err)
	}
	followUps, err := reader.SearchActivities(ctx, ActivitySearchInput{CustomerNo: customerNo, Kind: ActivityKindFollowUp, Limit: 10})
	if err != nil || len(followUps.Items) != 1 || followUps.Items[0].Status != "completed" {
		t.Fatalf("follow-up activity page=%+v err=%v", followUps, err)
	}
	blacklist, err := reader.SearchActivities(ctx, ActivitySearchInput{CustomerNo: customerNo, Kind: ActivityKindBlacklist, Limit: 10})
	if err != nil || len(blacklist.Items) != 1 || blacklist.Items[0].Status != "revoked" {
		t.Fatalf("blacklist activity page=%+v err=%v", blacklist, err)
	}
}
