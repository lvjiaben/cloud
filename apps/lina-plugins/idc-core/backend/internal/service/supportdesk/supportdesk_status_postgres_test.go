package supportdesk

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type postgresStatusAuditWriter struct {
	database gdb.DB
	fail     bool
}

func (w postgresStatusAuditWriter) AppendBusinessTransition(ctx context.Context, input idccap.BusinessAuditTransition) error {
	if gdb.TXFromCtx(ctx, w.database.GetGroup()) == nil {
		return gerror.New("status audit was not transaction-bound")
	}
	if w.fail {
		return errors.New("forced status audit failure")
	}
	before, err := json.Marshal(input.BeforeSnapshot)
	if err != nil {
		return err
	}
	after, err := json.Marshal(input.AfterSnapshot)
	if err != nil {
		return err
	}
	_, err = w.database.Exec(ctx, `INSERT INTO support_status_audit_probe (operation_code,target_no,required_permission,before_snapshot,after_snapshot) VALUES (?,?,?,?,?)`, input.OperationCode, input.TargetNo, input.RequiredPermission, before, after)
	return err
}

func (postgresStatusAuditWriter) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch status audit")
}

type scopedSupportDeskBizCtxStub struct{}

func (scopedSupportDeskBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 4}
}

type scopedSupportDeskAuthzStub struct{}

func (scopedSupportDeskAuthzStub) BatchGetPermissions(context.Context, []authz.PermissionKey) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}
func (scopedSupportDeskAuthzStub) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	result := make(map[authz.PermissionKey]bool, len(keys))
	for _, key := range keys {
		if key != authz.PermissionKey(idccap.HostPermissionWildcard) && key != authz.PermissionKey(PermissionViewAll) {
			result[key] = true
		}
	}
	return result, nil
}
func (scopedSupportDeskAuthzStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return true, nil
}
func (scopedSupportDeskAuthzStub) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return false, nil
}
func (scopedSupportDeskAuthzStub) ReplaceRolePermissions(context.Context, authz.RoleID, []authz.PermissionKey) error {
	return nil
}

func TestCustomStatusTransitionLifecyclePostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedStatusTransitionFixture(t, ctx, database)
	service := newAuthorizedSupportDeskTestService(database)
	service.audit = postgresStatusAuditWriter{database: database}

	detail, err := service.DetailStaff(ctx, "TKT_status")
	if err != nil {
		t.Fatal(err)
	}
	if !hasAction(detail.AvailableActions, ActionStatusTransition) || len(detail.AvailableStatusTransitions) != 4 {
		t.Fatalf("unexpected initial status actions=%v options=%+v", detail.AvailableActions, detail.AvailableStatusTransitions)
	}

	waiting, err := service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_waiting", TicketVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Ticket.BaseStage != StageWaitingCustomer || waiting.Ticket.CustomStatusNo != "STS_waiting" || waiting.Ticket.Version != 2 {
		t.Fatalf("unexpected waiting transition: %+v", waiting.Ticket)
	}
	assertStatusTransitionCounts(t, ctx, database, 1, 1, 1)

	if _, err = service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_working", TicketVersion: 1,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale status transition error=%v, want conflict", err)
	}
	assertStatusTransitionCounts(t, ctx, database, 1, 1, 1)

	working, err := service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_working", TicketVersion: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if working.Ticket.BaseStage != StageInProgress || working.Ticket.Version != 3 {
		t.Fatalf("unexpected working transition: %+v", working.Ticket)
	}
	activePauses, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_sla_pause WHERE ticket_no='TKT_status' AND ended_at IS NULL`)
	if err != nil || activePauses != 0 {
		t.Fatalf("active SLA pauses=%d err=%v, want zero", activePauses, err)
	}

	resolved, err := service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_resolved", TicketVersion: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Ticket.BaseStage != StageResolved || resolved.Ticket.ResolvedAt == nil || resolved.Ticket.Version != 4 {
		t.Fatalf("unexpected resolved transition: %+v", resolved.Ticket)
	}
	if _, err = service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_closed", TicketVersion: 4,
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("reasonless close error=%v, want invalid", err)
	}
	closed, err := service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_closed", Reason: "customer confirmed", TicketVersion: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if closed.Ticket.BaseStage != StageClosed || closed.Ticket.ClosedAt == nil || closed.Ticket.Version != 5 {
		t.Fatalf("unexpected closed transition: %+v", closed.Ticket)
	}

	reopened, err := service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_working", Reason: "continue investigation", TicketVersion: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Ticket.BaseStage != StageInProgress || reopened.Ticket.CurrentRound != 2 || reopened.Ticket.Version != 6 {
		t.Fatalf("unexpected reopened transition: %+v", reopened.Ticket)
	}
	rounds, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_sla_round WHERE ticket_no='TKT_status'`)
	if err != nil || rounds != 2 {
		t.Fatalf("SLA rounds=%d err=%v, want two", rounds, err)
	}
	assertStatusTransitionCounts(t, ctx, database, 5, 0, 5)

	var lastHistory StatusHistory
	if err = database.GetScan(ctx, &lastHistory, `SELECT custom_status_no,from_base_stage AS from_stage,to_base_stage AS to_stage,source_code AS source,reason_code,reason_text AS reason FROM plugin_idc_core_support_ticket_status_history WHERE ticket_no='TKT_status' ORDER BY id DESC LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if lastHistory.CustomStatusNo != "STS_working" || lastHistory.FromStage != StageClosed || lastHistory.ToStage != StageInProgress || lastHistory.ReasonCode != "custom_status_transition" {
		t.Fatalf("unexpected status history: %+v", lastHistory)
	}
}

func TestCustomStatusTransitionAuditFailureRollsBackPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedStatusTransitionFixture(t, ctx, database)
	service := newAuthorizedSupportDeskTestService(database)
	service.audit = postgresStatusAuditWriter{database: database, fail: true}

	_, err := service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_waiting", TicketVersion: 1,
	})
	if !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("audit failure error=%v, want unavailable", err)
	}
	var row Ticket
	if err = database.GetScan(ctx, &row, `SELECT base_stage,COALESCE(custom_status_no,'') AS custom_status_no,version FROM plugin_idc_core_support_ticket WHERE ticket_no='TKT_status'`); err != nil {
		t.Fatal(err)
	}
	if row.BaseStage != StageInProgress || row.CustomStatusNo != "" || row.Version != 1 {
		t.Fatalf("audit failure changed ticket: %+v", row)
	}
	assertStatusTransitionCounts(t, ctx, database, 0, 0, 0)
}

func TestCustomStatusTransitionRespectsPersonalDataScopePostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedStatusTransitionFixture(t, ctx, database)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET assignee_user_id=8 WHERE ticket_no='TKT_status'`); err != nil {
		t.Fatal(err)
	}
	service := newAuthorizedSupportDeskTestService(database)
	service.bizCtx = scopedSupportDeskBizCtxStub{}
	service.authorization = scopedSupportDeskAuthzStub{}
	service.audit = postgresStatusAuditWriter{database: database}

	_, err := service.TransitionStatus(ctx, StatusTransitionInput{
		TicketNo: "TKT_status", CustomStatusNo: "STS_waiting", TicketVersion: 1,
	})
	if !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("out-of-scope status transition error=%v, want not accessible", err)
	}
	assertStatusTransitionCounts(t, ctx, database, 0, 0, 0)
}

func seedStatusTransitionFixture(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE support_status_audit_probe (id BIGSERIAL PRIMARY KEY,operation_code TEXT NOT NULL,target_no TEXT NOT NULL,required_permission TEXT NOT NULL,before_snapshot JSONB NOT NULL,after_snapshot JSONB NOT NULL)`,
		`INSERT INTO plugin_idc_core_support_department (tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,version,created_by,updated_by) VALUES (0,'DEP_status','状态处理','Status handling','manual_claim','active',1,7,7)`,
		`INSERT INTO plugin_idc_core_support_custom_status (tenant_id,status_no,name_zh_cn,name_en_us,base_stage,sort_order,status_code,version,created_by,updated_by) VALUES (0,'STS_working','处理中','Working','in_progress',10,'active',1,7,7),(0,'STS_waiting','等客户','Waiting for customer','waiting_customer',20,'active',1,7,7),(0,'STS_resolved','已处理','Resolved','resolved',30,'active',1,7,7),(0,'STS_closed','已关闭','Closed','closed',40,'active',1,7,7)`,
		`INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,department_no,subject,base_stage,priority_code,current_round,last_activity_at,version,created_by,updated_by) VALUES (0,'TKT_status','internal','internal','DEP_status','Status transition','in_progress','normal',1,CURRENT_TIMESTAMP,1,7,7)`,
		`INSERT INTO plugin_idc_core_support_ticket_sla_round (tenant_id,ticket_no,round_no,created_at) VALUES (0,'TKT_status',1,CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
}

func assertStatusTransitionCounts(t *testing.T, ctx context.Context, database gdb.DB, history, activePause, audits int) {
	t.Helper()
	for name, check := range map[string]struct {
		query string
		want  int
	}{
		"history": {`SELECT COUNT(1) FROM plugin_idc_core_support_ticket_status_history WHERE ticket_no='TKT_status'`, history},
		"pause":   {`SELECT COUNT(1) FROM plugin_idc_core_support_ticket_sla_pause WHERE ticket_no='TKT_status' AND ended_at IS NULL`, activePause},
		"audit":   {`SELECT COUNT(1) FROM support_status_audit_probe WHERE target_no='TKT_status'`, audits},
	} {
		count, err := queryCount(ctx, database, check.query)
		if err != nil || count != check.want {
			t.Fatalf("%s count=%d err=%v, want %d", name, count, err, check.want)
		}
	}
}
