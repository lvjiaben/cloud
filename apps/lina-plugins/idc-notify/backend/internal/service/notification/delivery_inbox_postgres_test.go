package notification

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-notify/backend/cap/channelcap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

type recoveringInboxStub struct {
	mu      sync.Mutex
	request inboxcap.PublishRequest
	result  inboxcap.PublishResult
	calls   int
}

func (s *recoveringInboxStub) Publish(_ context.Context, request inboxcap.PublishRequest) (*inboxcap.PublishResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls == 1 {
		s.request = request
		s.result = inboxcap.PublishResult{MessageNo: "shared-message-1", EvidenceNo: "shared-evidence-1"}
		return nil, errors.New("simulated response loss after atomic publication")
	}
	if request != s.request {
		return nil, errors.New("idempotent inbox retry changed the request")
	}
	result := s.result
	result.Reused = true
	return &result, nil
}

func TestInAppDeliveryRecoversAnUnknownResponseWithoutDuplicatesPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	insertInAppDeliveryFixture(t, database, "inbox-task-1", 101)
	inbox := &recoveringInboxStub{}
	service := &serviceImpl{database: database, inbox: inbox}
	ownerUserID := 101
	task := &deliveryRow{
		TaskNo: "inbox-task-1", TopicCode: "support", ChannelCode: "in_app", OwnerUserID: &ownerUserID,
		SubjectContent: "Ticket update", BodyContent: "Your ticket has a new reply.",
		StatusCode: string(DeliverySending), RetrySequence: 0, Version: 1,
	}
	if err := service.deliverInApp(t.Context(), task); err == nil {
		t.Fatal("the simulated lost response must leave the leased task recoverable")
	}
	if err := service.deliverInApp(t.Context(), task); err != nil {
		t.Fatalf("repeat the exact shared inbox publication: %v", err)
	}
	var state struct {
		StatusCode              string `orm:"status_code"`
		ResultClass             string `orm:"result_class"`
		ExternalBoundaryCrossed bool   `orm:"external_boundary_crossed"`
		ProviderMessageRef      string `orm:"provider_message_ref"`
	}
	if err := database.Model(deliveryTable).Ctx(t.Context()).Where("tenant_id", 0).
		Where("task_no", task.TaskNo).Fields("status_code,result_class,external_boundary_crossed,provider_message_ref").
		Scan(&state); err != nil {
		t.Fatalf("read delivered task: %v", err)
	}
	if state.StatusCode != string(DeliveryDelivered) || state.ResultClass != string(channelcap.ResultDelivered) ||
		state.ExternalBoundaryCrossed || state.ProviderMessageRef != "shared-message-1" || inbox.calls != 2 {
		t.Fatalf("unexpected recovered delivery: state=%+v calls=%d", state, inbox.calls)
	}
	count, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_delivery_attempt
        WHERE tenant_id=0 AND task_no=? AND retry_sequence=0`, task.TaskNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("in-app recovery created duplicate attempts: count=%d err=%v", count.Int(), err)
	}
}

func insertInAppDeliveryFixture(t *testing.T, database gdb.DB, taskNo string, ownerUserID int) {
	t.Helper()
	chainNo := "chain-" + taskNo
	err := database.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
			return err
		}
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_notify_delivery_chain
            (tenant_id,chain_no,root_task_no,tail_task_no) VALUES (0,?,?,?)`, chainNo, taskNo, taskNo); err != nil {
			return err
		}
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_notify_delivery_task
            (tenant_id,task_no,chain_no,event_code,topic_code,channel_code,locale_code,owner_user_id,
             business_object_type,business_object_no,subject_content,body_content,content_digest,status_code)
            VALUES (0,?,?,'support.ticket_updated','support','in_app','en-US',?,
                    'support_ticket','ticket-inbox','Ticket update','Your ticket has a new reply.',?,'sending')`,
			taskNo, chainNo, ownerUserID, digestText("inbox-task", taskNo))
		return err
	})
	if err != nil {
		t.Fatalf("insert in-app delivery fixture: %v", err)
	}
}
