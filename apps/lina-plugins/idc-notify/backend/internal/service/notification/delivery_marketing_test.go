package notification

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

type marketingSendGateStub struct {
	decisions []notificationcap.MarketingSendDecision
	inputs    []notificationcap.MarketingSendInput
}

func (s *marketingSendGateStub) CheckSendBatch(
	_ context.Context,
	inputs []notificationcap.MarketingSendInput,
) ([]notificationcap.MarketingSendDecision, error) {
	s.inputs = append([]notificationcap.MarketingSendInput(nil), inputs...)
	return append([]notificationcap.MarketingSendDecision(nil), s.decisions...), nil
}

func TestEvaluateMarketingRechecksImmediatelyBeforeDelivery(t *testing.T) {
	evaluatedAt := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	gate := &marketingSendGateStub{decisions: []notificationcap.MarketingSendDecision{{
		CampaignNo: "campaign-1", SnapshotNo: "snapshot-1", CustomerNo: "customer-1",
		ContactNo: "contact-1", Channel: notificationcap.ChannelEmail,
		Allowed: true, DecisionDigest: "decision-digest", ConsentEventNo: "consent-event",
		ConsentVersion: 3, PreferenceVersion: 5, QuietHoursVersion: 2, EvaluatedAt: evaluatedAt,
	}}}
	service := &serviceImpl{sources: notificationcap.Runtime{Marketing: gate}}
	input := EnqueueInput{
		BusinessObjectType:  string(notificationcap.BusinessObjectMarketingCampaign),
		BusinessObjectNo:    "campaign-1",
		MarketingSnapshotNo: "snapshot-1",
		CustomerNo:          "customer-1",
		ContactNo:           "contact-1",
		ChannelCode:         string(notificationcap.ChannelEmail),
	}

	decision, err := service.evaluateMarketing(t.Context(), input, "content-digest")
	if err != nil {
		t.Fatalf("evaluate current marketing gate: %v", err)
	}
	if decision.DecisionDigest != "decision-digest" || decision.ConsentEventNo != "consent-event" ||
		decision.PreferenceVersion != 5 || !decision.EvaluatedAt.Equal(evaluatedAt) {
		t.Fatalf("marketing decision evidence was not preserved: %+v", decision)
	}
	if len(gate.inputs) != 1 {
		t.Fatalf("expected one bounded gate input, got %+v", gate.inputs)
	}
	got := gate.inputs[0]
	if got.CampaignNo != "campaign-1" || got.SnapshotNo != "snapshot-1" ||
		got.CustomerNo != "customer-1" || got.ContactNo != "contact-1" ||
		got.Channel != notificationcap.ChannelEmail || got.ContentDigest != "content-digest" {
		t.Fatalf("unexpected marketing gate input: %+v", got)
	}
}

func TestEvaluateMarketingFailsClosed(t *testing.T) {
	gate := &marketingSendGateStub{decisions: []notificationcap.MarketingSendDecision{{
		Allowed: false, DecisionDigest: "denied-decision",
	}}}
	service := &serviceImpl{sources: notificationcap.Runtime{Marketing: gate}}
	input := EnqueueInput{
		BusinessObjectType:  string(notificationcap.BusinessObjectMarketingCampaign),
		BusinessObjectNo:    "campaign-1",
		MarketingSnapshotNo: "snapshot-1",
		CustomerNo:          "customer-1",
		ContactNo:           "contact-1",
		ChannelCode:         string(notificationcap.ChannelSMS),
	}

	if _, err := service.evaluateMarketing(t.Context(), input, "content-digest"); err == nil {
		t.Fatal("denied marketing decision must block delivery")
	}
	input.MarketingSnapshotNo = ""
	gate.inputs = nil
	if _, err := service.evaluateMarketing(t.Context(), input, "content-digest"); err == nil {
		t.Fatal("missing marketing snapshot must fail closed")
	}
	if len(gate.inputs) != 0 {
		t.Fatalf("invalid marketing input must be rejected before the gate call: %+v", gate.inputs)
	}
}

type marketingInboxStub struct{ calls int }

func (s *marketingInboxStub) Publish(_ context.Context, _ inboxcap.PublishRequest) (*inboxcap.PublishResult, error) {
	s.calls++
	return &inboxcap.PublishResult{MessageNo: "unexpected-message", EvidenceNo: "unexpected-evidence"}, nil
}

func TestProcessClaimedRechecksMarketingBeforeInAppAndExternalDeliveryPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel
SET status_code='enabled',provider_code=CASE WHEN channel_code='signed_webhook' THEN 'marketing-test-provider' ELSE provider_code END
WHERE tenant_id=0 AND channel_code IN ('in_app','signed_webhook')`); err != nil {
		t.Fatalf("enable delivery channels: %v", err)
	}
	mismatches := []struct {
		name   string
		mutate func(*notificationcap.MarketingSendDecision)
	}{
		{name: "campaign", mutate: func(value *notificationcap.MarketingSendDecision) { value.CampaignNo = "wrong-campaign" }},
		{name: "snapshot", mutate: func(value *notificationcap.MarketingSendDecision) { value.SnapshotNo = "wrong-snapshot" }},
		{name: "customer", mutate: func(value *notificationcap.MarketingSendDecision) { value.CustomerNo = "wrong-customer" }},
		{name: "contact", mutate: func(value *notificationcap.MarketingSendDecision) { value.ContactNo = "wrong-contact" }},
		{name: "channel", mutate: func(value *notificationcap.MarketingSendDecision) { value.Channel = notificationcap.ChannelEmail }},
	}
	for _, channelCode := range []string{string(notificationcap.ChannelInApp), "signed_webhook"} {
		for _, mismatch := range mismatches {
			t.Run(channelCode+"/"+mismatch.name, func(t *testing.T) {
				taskNo := "marketing-final-gate-" + channelCode + "-" + mismatch.name
				task := insertMarketingDeliveryFixture(t, database, taskNo, channelCode)
				decision := notificationcap.MarketingSendDecision{
					CampaignNo: "campaign-1", SnapshotNo: "snapshot-1", CustomerNo: "customer-1",
					ContactNo: "contact-1", Channel: notificationcap.Channel(channelCode),
					Allowed: true, DecisionDigest: "mismatched-at-final-delivery",
				}
				mismatch.mutate(&decision)
				gate := &marketingSendGateStub{decisions: []notificationcap.MarketingSendDecision{decision}}
				inbox := &marketingInboxStub{}
				adapter := &wecomDeliveryAdapterStub{}
				service := &serviceImpl{
					database: database, inbox: inbox,
					sources:  notificationcap.Runtime{Marketing: gate},
					adapters: map[string]adapterBinding{"marketing-test-provider": {adapter: adapter}},
				}

				if err := service.processClaimed(t.Context(), task); err != nil {
					t.Fatalf("finish denied marketing delivery: %v", err)
				}
				if len(gate.inputs) != 1 || gate.inputs[0].Channel != notificationcap.Channel(channelCode) {
					t.Fatalf("final marketing gate was not called for %s: %+v", channelCode, gate.inputs)
				}
				if inbox.calls != 0 || adapter.calls != 0 {
					t.Fatalf("denied marketing crossed a delivery adapter: inbox=%d provider=%d", inbox.calls, adapter.calls)
				}
				var state struct {
					StatusCode              string `orm:"status_code"`
					ExternalBoundaryCrossed bool   `orm:"external_boundary_crossed"`
				}
				if err := database.Model(deliveryTable).Ctx(t.Context()).
					Where("tenant_id", 0).Where("task_no", task.TaskNo).
					Fields("status_code,external_boundary_crossed").Scan(&state); err != nil {
					t.Fatalf("read denied marketing task: %v", err)
				}
				if state.StatusCode != string(DeliveryPreferenceBlocked) || state.ExternalBoundaryCrossed {
					t.Fatalf("denied marketing crossed a delivery boundary: %+v", state)
				}
				attempts, err := database.Model(deliveryAttemptTable).Ctx(t.Context()).
					Where("tenant_id", 0).Where("task_no", task.TaskNo).Count()
				if err != nil || attempts != 0 {
					t.Fatalf("denied marketing created delivery attempts: count=%d err=%v", attempts, err)
				}
			})
		}
	}
}

func TestEnqueueRejectsMarketingChannelsBeforeCreatingTaskPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	applyMarketingDeliveryProviderMigration(t, database)
	channels := []string{
		string(notificationcap.ChannelInApp), string(notificationcap.ChannelEmail),
		string(notificationcap.ChannelSMS), string(notificationcap.ChannelWeChatOfficial),
		"signed_webhook",
	}
	for _, channelCode := range channels {
		t.Run(channelCode, func(t *testing.T) {
			templateNo := insertMarketingEnqueueTemplate(t, database, channelCode)
			gate := &marketingSendGateStub{decisions: []notificationcap.MarketingSendDecision{{
				CampaignNo: "campaign-1", SnapshotNo: "snapshot-1", CustomerNo: "customer-1",
				ContactNo: "contact-1", Channel: notificationcap.Channel(channelCode),
				Allowed: false, DecisionDigest: "denied-at-enqueue",
			}}}
			service := &serviceImpl{database: database, sources: notificationcap.Runtime{
				Marketing: gate, Contacts: materializationContactResolverStub{},
			}}
			input := EnqueueInput{
				EventCode: "marketing.campaign", ChannelCode: channelCode, Locale: "en-US",
				OwnerUserID: 101, CustomerNo: "customer-1", ContactNo: "contact-1",
				BusinessObjectType: string(notificationcap.BusinessObjectMarketingCampaign), BusinessObjectNo: "campaign-1",
				MarketingSnapshotNo: "snapshot-1", TemplateNo: templateNo, TemplateVersion: 99,
				Subject: "Campaign", Body: "Campaign body", Variables: map[string]string{"campaign_name": "Campaign"},
				IdempotencyKey: "marketing-denied-" + channelCode,
			}
			if _, err := service.Enqueue(t.Context(), input); err == nil {
				t.Fatalf("denied %s marketing must not be queued", channelCode)
			}
			if len(gate.inputs) != 1 || gate.inputs[0].Channel != notificationcap.Channel(channelCode) {
				t.Fatalf("%s enqueue did not call the marketing gate: %+v", channelCode, gate.inputs)
			}
			for _, table := range []string{deliveryTable, deliveryChainTable, deliveryAttemptTable} {
				count, err := database.Model(table).Ctx(t.Context()).Where("tenant_id", 0).Count()
				if err != nil || count != 0 {
					t.Fatalf("denied %s enqueue wrote %s: count=%d err=%v", channelCode, table, count, err)
				}
			}
		})
	}
}

func applyMarketingDeliveryProviderMigration(t *testing.T, database gdb.DB) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve marketing delivery test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "006-multi-provider-sms.sql"))
	if err != nil {
		t.Fatalf("read multi-provider SMS migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(content)); err != nil {
		t.Fatalf("apply multi-provider SMS migration: %v", err)
	}
}

func insertMarketingEnqueueTemplate(t *testing.T, database gdb.DB, channelCode string) string {
	t.Helper()
	templateNo := "marketing-enqueue-" + channelCode
	providerCode := ""
	if channelCode == string(notificationcap.ChannelSMS) {
		providerCode = "tencent_cloud_sms_v20210111"
	}
	if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel
SET status_code='enabled',provider_code=NULLIF(?, '') WHERE tenant_id=0 AND channel_code=?`,
		providerCode, channelCode); err != nil {
		t.Fatalf("enable %s delivery channel: %v", channelCode, err)
	}
	if _, err := database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_template_version
    (tenant_id,template_no,version,event_code,channel_code,locale_code,subject_template,
     body_template,provider_code,status_code,content_digest,created_by)
VALUES (0,?,99,'marketing.campaign',?,'en-US','{{campaign_name}}','Campaign body',?,
        'active',?,101)`, templateNo, channelCode, providerCode, digestText(templateNo)); err != nil {
		t.Fatalf("insert %s marketing template: %v", channelCode, err)
	}
	return templateNo
}

func insertMarketingDeliveryFixture(t *testing.T, database gdb.DB, taskNo, channelCode string) *deliveryRow {
	t.Helper()
	chainNo := "chain-" + taskNo
	ownerUserID := 101
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
             customer_no,contact_no,business_object_type,business_object_no,marketing_snapshot_no,
             subject_content,body_content,content_digest,status_code)
            VALUES (0,?,?,'marketing.campaign','marketing',?,'en-US',?,
                    'customer-1','contact-1','marketing_campaign','campaign-1','snapshot-1',
                    'Campaign update','Campaign body',?,'sending')`,
			taskNo, chainNo, channelCode, ownerUserID, digestText("marketing-final-gate", taskNo))
		return err
	})
	if err != nil {
		t.Fatalf("insert marketing delivery fixture: %v", err)
	}
	return &deliveryRow{
		TaskNo: taskNo, ChainNo: chainNo, EventCode: "marketing.campaign", TopicCode: "marketing",
		ChannelCode: channelCode, LocaleCode: "en-US", OwnerUserID: &ownerUserID,
		CustomerNo: "customer-1", ContactNo: "contact-1", BusinessObjectType: "marketing_campaign",
		BusinessObjectNo: "campaign-1", MarketingSnapshotNo: "snapshot-1",
		SubjectContent: "Campaign update", BodyContent: "Campaign body",
		ContentDigest: digestText("marketing-final-gate", taskNo), StatusCode: string(DeliverySending), Version: 1,
	}
}
