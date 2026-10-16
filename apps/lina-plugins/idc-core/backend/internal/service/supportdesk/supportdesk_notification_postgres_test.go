package supportdesk

import (
	"context"
	"encoding/json"
	"strconv"
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

// TestPersonalTargetsAreRejected verifies removed personal routing cannot be
// reached by keeping old request fields in a manually crafted payload.
func TestPersonalTargetsAreRejectedPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	if err := service.Transfer(ctx, TransferInput{TicketNo: "TKT_target", DepartmentNo: "DEP_target", AssigneeUserID: 77, TicketVersion: 1, Reason: "handoff"}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("transfer err=%v", err)
	}
	if _, err := service.Escalate(ctx, EscalateInput{TicketNo: "TKT_target", ToUserID: 77, TicketVersion: 1, Reason: "escalate"}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("escalate err=%v", err)
	}
}

// TestAccountRatingsPersistWithoutStaffNotifications verifies customers can
// submit every score while low ratings enter follow-up without changing owner.
func TestAccountRatingsPersistWithoutStaffNotificationsPostgreSQL(t *testing.T) {
	for _, stars := range []int{1, 2, 3, 4, 5} {
		t.Run(strconv.Itoa(stars), func(t *testing.T) {
			ctx, database := newSupportDeskTestDatabase(t)
			if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,display_name) VALUES (0,'CUS_rating','Rating')`); err != nil {
				t.Fatal(err)
			}
			seedSupportNotificationTicket(t, ctx, database, "TKT_rating", "CUS_rating", 77)
			if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage='resolved' WHERE ticket_no='TKT_rating'`); err != nil {
				t.Fatal(err)
			}
			service := newCustomerSupportDeskTestService(database, "CUS_rating")
			detail, err := service.RateAccount(ctx, RatingInput{TicketNo: "TKT_rating", RoundNo: 1, Stars: stars, Feedback: "feedback", TicketVersion: 1})
			if err != nil {
				t.Fatal(err)
			}
			if detail.Rating == nil || detail.Rating.Stars != stars || detail.Ticket.FollowUpRequired != (stars <= 2) || detail.Ticket.AssigneeUserID != 77 || detail.Ticket.BaseStage != StageResolved {
				t.Fatalf("unexpected rating: %+v %+v", detail.Rating, detail.Ticket)
			}
			if _, err = service.RateAccount(ctx, RatingInput{TicketNo: "TKT_rating", RoundNo: 1, Stars: stars, TicketVersion: detail.Ticket.Version}); !bizerr.Is(err, CodeConflict) {
				t.Fatalf("duplicate rating=%v", err)
			}
		})
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
