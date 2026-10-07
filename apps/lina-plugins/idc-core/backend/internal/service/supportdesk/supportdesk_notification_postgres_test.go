package supportdesk

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

type postgresSupportNotificationSink struct {
	database gdb.DB
	fail     bool
}

func (s postgresSupportNotificationSink) Enqueue(ctx context.Context, request notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return notificationcap.EventReceipt{}, gerror.New("support notification was not transaction-bound")
	}
	if s.fail {
		return notificationcap.EventReceipt{}, gerror.New("forced support notification failure")
	}
	variables, err := json.Marshal(request.Variables)
	if err != nil {
		return notificationcap.EventReceipt{}, err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO support_notification_probe
        (event_code,customer_no,contact_no,ticket_no,business_version,variables)
        VALUES (?,?,?,?,?,?::jsonb)`, request.EventCode, request.Recipient.CustomerNo,
		request.Recipient.ContactNo, request.BusinessObjectNo, request.BusinessObjectVersion, variables)
	return notificationcap.EventReceipt{}, err
}

func TestPublicReplyPersistsAuthoritativeCustomerIntentInSameTransactionPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedSupportNotificationTables(t, ctx, database)
	seedSupportNotificationCustomer(t, ctx, database, "CUS_notify", "CON_primary", true)
	seedSupportNotificationTicket(t, ctx, database, "TKT_notify", "CUS_notify", 7)
	service := newAuthorizedSupportDeskTestService(database)
	service.notifications = postgresSupportNotificationSink{database: database}

	detail, err := service.ReplyStaff(ctx, MessageInput{
		TicketNo: "TKT_notify", TicketVersion: 1, Body: "A public update",
	})
	if err != nil {
		t.Fatalf("reply with notification intent: %v", err)
	}
	if detail.Ticket.Version != 2 {
		t.Fatalf("ticket version=%d, want 2", detail.Ticket.Version)
	}
	var rows []struct {
		EventCode       string          `orm:"event_code"`
		CustomerNo      string          `orm:"customer_no"`
		ContactNo       string          `orm:"contact_no"`
		TicketNo        string          `orm:"ticket_no"`
		BusinessVersion int64           `orm:"business_version"`
		Variables       json.RawMessage `orm:"variables"`
	}
	if err = database.GetScan(ctx, &rows, `SELECT event_code,customer_no,contact_no,ticket_no,business_version,variables FROM support_notification_probe`); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].EventCode != supportEventTicketPublicUpdated ||
		rows[0].CustomerNo != "CUS_notify" || rows[0].ContactNo != "CON_primary" ||
		rows[0].TicketNo != "TKT_notify" || rows[0].BusinessVersion != 2 {
		t.Fatalf("unexpected notification probe rows: %+v", rows)
	}
	var variables map[string]string
	if err = json.Unmarshal(rows[0].Variables, &variables); err != nil {
		t.Fatal(err)
	}
	if variables["ticket_no"] != "TKT_notify" || variables["title"] != "Customer ticket" {
		t.Fatalf("unexpected safe variables: %+v", variables)
	}
}

func TestMissingPrimaryContactDoesNotGuessNotificationRecipientPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedSupportNotificationTables(t, ctx, database)
	seedSupportNotificationCustomer(t, ctx, database, "CUS_no_primary", "CON_technical", false)
	seedSupportNotificationTicket(t, ctx, database, "TKT_no_primary", "CUS_no_primary", 7)
	service := newAuthorizedSupportDeskTestService(database)
	service.notifications = postgresSupportNotificationSink{database: database}

	if _, err := service.ReplyStaff(ctx, MessageInput{TicketNo: "TKT_no_primary", TicketVersion: 1, Body: "Public update"}); err != nil {
		t.Fatalf("reply without primary contact: %v", err)
	}
	count, err := queryCount(ctx, database, `SELECT COUNT(1) FROM support_notification_probe`)
	if err != nil || count != 0 {
		t.Fatalf("guessed notification recipient count=%d err=%v", count, err)
	}
}

func TestNotificationFailureRollsBackPublicReplyPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedSupportNotificationTables(t, ctx, database)
	seedSupportNotificationCustomer(t, ctx, database, "CUS_rollback", "CON_rollback", true)
	seedSupportNotificationTicket(t, ctx, database, "TKT_rollback", "CUS_rollback", 7)
	service := newAuthorizedSupportDeskTestService(database)
	service.notifications = postgresSupportNotificationSink{database: database, fail: true}

	_, err := service.ReplyStaff(ctx, MessageInput{TicketNo: "TKT_rollback", TicketVersion: 1, Body: "Must roll back"})
	if !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("reply error=%v, want unavailable", err)
	}
	version, err := database.GetValue(ctx, `SELECT version FROM plugin_idc_core_support_ticket WHERE ticket_no='TKT_rollback'`)
	if err != nil || version.Int64() != 1 {
		t.Fatalf("ticket version after rollback=%d err=%v", version.Int64(), err)
	}
	count, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_message WHERE ticket_no='TKT_rollback'`)
	if err != nil || count != 0 {
		t.Fatalf("message count after rollback=%d err=%v", count, err)
	}
}

func TestTargetStaffActionsFailClosedWithoutExactTargetAuthorityPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_department
        (tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,created_by,updated_by)
        VALUES (0,'DEP_auto','Auto','Auto','round_robin','active',7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_department_member
        (tenant_id,department_no,user_id,role_code,status_code) VALUES (0,'DEP_auto',77,'leader','active')`); err != nil {
		t.Fatal(err)
	}
	service := newAuthorizedSupportDeskTestService(database)
	var assignee int64
	var mode string
	err := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var resolveErr error
		assignee, mode, resolveErr = service.resolveInitialAssignee(txCtx, "DEP_auto")
		return resolveErr
	})
	if err != nil || assignee != 0 || mode != "round_robin" {
		t.Fatalf("automatic assignment assignee/mode=%d/%s err=%v", assignee, mode, err)
	}

	seedSupportNotificationTicket(t, ctx, database, "TKT_target_staff", "", 7)
	if _, err = service.Transfer(ctx, TransferInput{
		TicketNo: "TKT_target_staff", TicketVersion: 1, DepartmentNo: "DEP_auto", AssigneeUserID: 77, Reason: "handoff",
	}); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("specific transfer error=%v, want unavailable", err)
	}
	if _, err = service.Escalate(ctx, EscalateInput{
		TicketNo: "TKT_target_staff", TicketVersion: 1, ToUserID: 77, Reason: "escalate",
	}); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("escalation error=%v, want unavailable", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage='closed',closed_at=CURRENT_TIMESTAMP WHERE ticket_no='TKT_target_staff'`); err != nil {
		t.Fatal(err)
	}
	staffDetail, err := service.DetailStaff(ctx, "TKT_target_staff")
	if err != nil {
		t.Fatal(err)
	}
	if hasAction(staffDetail.AvailableActions, "reopen") {
		t.Fatalf("assigned ticket exposed unavailable reopen action: %v", staffDetail.AvailableActions)
	}
	if _, err = service.ReopenStaff(ctx, TransitionInput{
		TicketNo: "TKT_target_staff", TicketVersion: 1, Reason: "reopen",
	}); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("assigned reopen error=%v, want unavailable", err)
	}
}

func TestAccountLowAndNeutralRatingsFailBeforeWritingWithoutExactStaffRecipientsPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject
        (tenant_id,customer_no,display_name) VALUES (0,'CUS_rating','Rating customer')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket
        (tenant_id,ticket_no,ticket_type,visibility_code,customer_no,assignee_user_id,
         subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by)
        VALUES (0,'TKT_rating','customer','customer','CUS_rating',77,
                'Rate me','resolved','normal',1,CURRENT_TIMESTAMP,8,8)`); err != nil {
		t.Fatal(err)
	}
	service := newCustomerSupportDeskTestService(database, "CUS_rating")

	for _, stars := range []int{1, 2, 3} {
		_, err := service.RateAccount(ctx, RatingInput{
			TicketNo: "TKT_rating", RoundNo: 1, Stars: stars,
			Feedback: "must not partially persist", TicketVersion: 1,
		})
		if !bizerr.Is(err, CodeUnavailable) {
			t.Fatalf("stars=%d error=%v, want unavailable", stars, err)
		}
	}
	ratings, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_rating WHERE ticket_no='TKT_rating'`)
	if err != nil || ratings != 0 {
		t.Fatalf("partial rating rows=%d err=%v", ratings, err)
	}
	var row struct {
		FollowUp bool  `orm:"follow_up_required"`
		Version  int64 `orm:"version"`
	}
	if err = database.GetScan(ctx, &row, `SELECT follow_up_required,version FROM plugin_idc_core_support_ticket WHERE ticket_no='TKT_rating'`); err != nil {
		t.Fatal(err)
	}
	if row.FollowUp || row.Version != 1 {
		t.Fatalf("rating failure changed ticket: %+v", row)
	}
}

func seedSupportNotificationTables(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE plugin_idc_core_customer_contact (
            id BIGSERIAL PRIMARY KEY, tenant_id BIGINT NOT NULL DEFAULT 0,
            contact_no VARCHAR(40) NOT NULL UNIQUE, customer_no VARCHAR(40) NOT NULL,
            status_code VARCHAR(16) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_contact_role (
            id BIGSERIAL PRIMARY KEY, tenant_id BIGINT NOT NULL DEFAULT 0,
            customer_no VARCHAR(40) NOT NULL, contact_no VARCHAR(40) NOT NULL,
            role_code VARCHAR(24) NOT NULL, status_code VARCHAR(16) NOT NULL)`,
		`CREATE TABLE support_notification_probe (
            id BIGSERIAL PRIMARY KEY, event_code VARCHAR(96) NOT NULL,
            customer_no VARCHAR(40) NOT NULL, contact_no VARCHAR(40) NOT NULL,
            ticket_no VARCHAR(40) NOT NULL, business_version BIGINT NOT NULL,
            variables JSONB NOT NULL)`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
}

func seedSupportNotificationCustomer(t *testing.T, ctx context.Context, database gdb.DB, customerNo, contactNo string, primary bool) {
	t.Helper()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,display_name) VALUES (0,?,?)`, customerNo, "Customer"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact (tenant_id,contact_no,customer_no,status_code) VALUES (0,?,?,'active')`, contactNo, customerNo); err != nil {
		t.Fatal(err)
	}
	role := "technical"
	if primary {
		role = "primary"
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact_role (tenant_id,customer_no,contact_no,role_code,status_code) VALUES (0,?,?,?,'active')`, customerNo, contactNo, role); err != nil {
		t.Fatal(err)
	}
}

func seedSupportNotificationTicket(t *testing.T, ctx context.Context, database gdb.DB, ticketNo, customerNo string, assignee int64) {
	t.Helper()
	ticketType, visibility := "internal", "internal"
	var customer any
	if customerNo != "" {
		ticketType, visibility, customer = "customer", "customer", customerNo
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket
        (tenant_id,ticket_no,ticket_type,visibility_code,customer_no,assignee_user_id,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by)
        VALUES (0,?,?,?,?,?,'Customer ticket','in_progress','normal',1,CURRENT_TIMESTAMP,7,7)`,
		ticketNo, ticketType, visibility, customer, nullableInt(assignee)); err != nil {
		t.Fatal(err)
	}
}
