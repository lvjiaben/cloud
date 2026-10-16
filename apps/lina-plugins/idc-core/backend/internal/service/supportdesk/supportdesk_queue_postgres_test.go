// This file exercises live role/organization queue ownership and queue-only
// escalation against isolated PostgreSQL schemas, without host private tables.

package supportdesk

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/orgcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type queueOrgStub struct {
	orgcap.Service
	deptID   int
	disabled bool
}

func (q *queueOrgStub) Available(context.Context) bool { return true }
func (q *queueOrgStub) Assignment() orgcap.AssignmentService {
	return queueAssignmentStub{deptID: q.deptID}
}
func (q *queueOrgStub) Department() orgcap.DepartmentService {
	return queueDepartmentStub{disabled: q.disabled}
}

type queueAssignmentStub struct {
	orgcap.AssignmentService
	deptID int
}

func (q queueAssignmentStub) ListByUser(_ context.Context, id int) (*orgcap.UserOrgProfile, error) {
	return &orgcap.UserOrgProfile{UserID: id, DeptID: q.deptID}, nil
}

type queueDepartmentStub struct {
	orgcap.DepartmentService
	disabled bool
}

func (q queueDepartmentStub) Get(_ context.Context, id int) (*orgcap.DeptInfo, error) {
	status := 1
	if q.disabled {
		status = 0
	}
	return &orgcap.DeptInfo{DeptID: id, Status: status}, nil
}

// BatchGet returns detached display projections for explicitly requested departments.
func (queueDepartmentStub) BatchGet(_ context.Context, ids []int) (*capmodel.BatchResult[*orgcap.DeptInfo, int], error) {
	r := &capmodel.BatchResult[*orgcap.DeptInfo, int]{Items: map[int]*orgcap.DeptInfo{}}
	for _, id := range ids {
		r.Items[id] = &orgcap.DeptInfo{DeptID: id, Status: 1, DeptName: "Organization"}
	}
	return r, nil
}

type queueBizCtxStub struct{ scope int }

func (q queueBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	scope := q.scope
	if scope == 0 {
		scope = 3
	}
	return bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: scope}
}

type queueAuthStub struct {
	supportDeskAuthzStub
	denyReply bool
}

func (q queueAuthStub) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	r := map[authz.PermissionKey]bool{}
	for _, k := range keys {
		r[k] = k != PermissionViewAll && k != authz.PermissionKey(idccap.HostPermissionWildcard) && !(q.denyReply && k == PermissionReply)
	}
	return r, nil
}

// TestQueueClaimTracksOrganizationAndRole verifies loss of department or reply
// permission cannot leave old assignment facts as authorization bypasses.
func TestQueueClaimTracksOrganizationAndRolePostgreSQL(t *testing.T) {
	ctx, db := newSupportDeskTestDatabase(t)
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_support_department(tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,created_by,updated_by,org_dept_id) VALUES(0,'DEP_queue','Queue','Queue','manual_claim','active',7,7,11)`); err != nil {
		t.Fatal(err)
	}
	seedSupportNotificationTicket(t, ctx, db, "TKT_queue", "", 0)
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET department_no='DEP_queue',base_stage='new' WHERE ticket_no='TKT_queue'`); err != nil {
		t.Fatal(err)
	}
	svc := newAuthorizedSupportDeskTestService(db)
	org := &queueOrgStub{deptID: 11}
	svc.organization = org
	svc.bizCtx = queueBizCtxStub{}
	svc.authorization = queueAuthStub{denyReply: true}
	if _, err := svc.Claim(ctx, ClaimInput{TicketNo: "TKT_queue", TicketVersion: 1}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("claim without reply=%v", err)
	}
	svc.authorization = queueAuthStub{}
	departments, deptErr := svc.actorDepartments(ctx, 7)
	if deptErr != nil || len(departments) != 1 || departments[0] != "DEP_queue" {
		t.Fatalf("departments=%#v err=%v", departments, deptErr)
	}
	detail, err := svc.Claim(ctx, ClaimInput{TicketNo: "TKT_queue", TicketVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Ticket.AssigneeUserID != 7 || detail.Ticket.BaseStage != StageInProgress {
		t.Fatalf("claim=%+v", detail.Ticket)
	}
	if _, err = svc.Claim(ctx, ClaimInput{TicketNo: "TKT_queue", TicketVersion: 1}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("repeat claim=%v", err)
	}
	svc.authorization = queueAuthStub{denyReply: true}
	if _, err = svc.ReplyStaff(ctx, MessageInput{TicketNo: "TKT_queue", TicketVersion: 2, Body: "permission revoked"}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("revoked reply grant still works=%v", err)
	}
	svc.authorization = queueAuthStub{}
	org.deptID = 12
	if _, err = svc.DetailStaff(ctx, "TKT_queue"); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("moved employee can still read=%v", err)
	}
	if _, err = svc.ReplyStaff(ctx, MessageInput{TicketNo: "TKT_queue", TicketVersion: 2, Body: "must reject"}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("moved employee can still reply=%v", err)
	}
}

// TestEscalationQueuesFollowUpWithoutIndividualRecipient verifies no employee
// notification proof is needed and base stage/assignee/SLA semantics stay intact.
func TestEscalationQueuesFollowUpWithoutIndividualRecipientPostgreSQL(t *testing.T) {
	ctx, db := newSupportDeskTestDatabase(t)
	seedSupportNotificationTicket(t, ctx, db, "TKT_escalate", "", 7)
	svc := newAuthorizedSupportDeskTestService(db)
	detail, err := svc.Escalate(ctx, EscalateInput{TicketNo: "TKT_escalate", TicketVersion: 1, Reason: "Need supervisor review"})
	if err != nil {
		t.Fatal(err)
	}
	if !detail.Ticket.FollowUpRequired || detail.Ticket.AssigneeUserID != 7 || detail.Ticket.BaseStage != StageInProgress || len(detail.Escalations) != 1 || detail.Escalations[0].ToUserID != 0 {
		t.Fatalf("escalation=%+v history=%+v", detail.Ticket, detail.Escalations)
	}
}

// TestTransferSuccessDoesNotReadBeyondNewScope covers a supervisor returning a
// ticket to another queue, then losing visibility as part of the successful write.
func TestTransferSuccessDoesNotReadBeyondNewScopePostgreSQL(t *testing.T) {
	ctx, db := newSupportDeskTestDatabase(t)
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_support_department(tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,created_by,updated_by,org_dept_id) VALUES(0,'DEP_source','Source','Source','manual_claim','active',7,7,11),(0,'DEP_target','Target','Target','manual_claim','active',7,7,12)`); err != nil {
		t.Fatal(err)
	}
	seedSupportNotificationTicket(t, ctx, db, "TKT_transfer", "", 77)
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET department_no='DEP_source' WHERE ticket_no='TKT_transfer'`); err != nil {
		t.Fatal(err)
	}
	svc := newAuthorizedSupportDeskTestService(db)
	svc.organization = &queueOrgStub{deptID: 11}
	svc.bizCtx = queueBizCtxStub{scope: 2}
	svc.authorization = queueAuthStub{}
	if err := svc.Transfer(ctx, TransferInput{TicketNo: "TKT_transfer", DepartmentNo: "DEP_target", Reason: "Another team owns this", TicketVersion: 1}); err != nil {
		t.Fatal(err)
	}
	var row struct {
		Department string
		Assignee   int64
		Version    int64
	}
	if err := db.GetScan(ctx, &row, `SELECT department_no AS department,COALESCE(assignee_user_id,0) AS assignee,version FROM plugin_idc_core_support_ticket WHERE ticket_no='TKT_transfer'`); err != nil {
		t.Fatal(err)
	}
	if row.Department != "DEP_target" || row.Assignee != 0 || row.Version != 2 {
		t.Fatalf("transfer=%+v", row)
	}
	if _, err := svc.DetailStaff(ctx, "TKT_transfer"); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("source team retained visibility=%v", err)
	}
	count, err := queryCount(ctx, db, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_assignment_history WHERE ticket_no='TKT_transfer' AND action_code='transferred' AND from_user_id=77 AND to_user_id IS NULL`)
	if err != nil || count != 1 {
		t.Fatalf("transfer history=%d err=%v", count, err)
	}
}

// noQueuePermissionStub deliberately keeps legacy IsPlatformAdmin=true while
// withholding role grants, reproducing the former all-data permission bypass.
type noQueuePermissionStub struct{ supportDeskAuthzStub }

func (noQueuePermissionStub) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{}, nil
}
func (noQueuePermissionStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}
func TestAllDataRoleDoesNotBypassTicketPermission(t *testing.T) {
	svc := newAuthorizedSupportDeskTestService(nil)
	svc.authorization = noQueuePermissionStub{}
	if _, err := svc.staffActor(context.Background(), PermissionClaim); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("all-data role without claim grant=%v", err)
	}
}

// TestInternalCreationRejectsInactiveOrForeignOrganization verifies staff cannot
// create work in a disabled organization or another department using a guessed queue.
func TestInternalCreationRejectsInactiveOrForeignOrganizationPostgreSQL(t *testing.T) {
	ctx, db := newSupportDeskTestDatabase(t)
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_support_department(tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,created_by,updated_by,org_dept_id) VALUES(0,'DEP_create','Create','Create','manual_claim','active',7,7,11)`); err != nil {
		t.Fatal(err)
	}
	svc := newAuthorizedSupportDeskTestService(db)
	svc.organization = &queueOrgStub{deptID: 11, disabled: true}
	input := CreateInput{DepartmentNo: "DEP_create", Subject: "Forbidden internal creation", Body: "must not persist"}
	if _, err := svc.CreateStaff(ctx, input); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("disabled organization creation=%v", err)
	}
	svc.organization = &queueOrgStub{deptID: 12}
	svc.bizCtx = queueBizCtxStub{}
	svc.authorization = queueAuthStub{}
	if _, err := svc.CreateStaff(ctx, input); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign organization creation=%v", err)
	}
	count, err := queryCount(ctx, db, `SELECT COUNT(*) FROM plugin_idc_core_support_ticket`)
	if err != nil || count != 0 {
		t.Fatalf("forbidden creation persisted=%d err=%v", count, err)
	}
}
