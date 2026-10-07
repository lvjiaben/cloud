package notification

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type callbackAdapterStub struct{ event channelcap.CallbackEvent }

func (callbackAdapterStub) ValidateConfiguration(context.Context, map[string]string, map[string][]byte) error {
	return nil
}
func (callbackAdapterStub) ValidateDestination([]byte) error { return nil }
func (callbackAdapterStub) Deliver(context.Context, channelcap.Delivery) (channelcap.Result, error) {
	return channelcap.Result{}, nil
}
func (callbackAdapterStub) Query(context.Context, channelcap.Query) (channelcap.Result, error) {
	return channelcap.Result{}, nil
}
func (callbackAdapterStub) VerifyChallenge(context.Context, channelcap.CallbackInput) (string, error) {
	return "challenge", nil
}
func (stub callbackAdapterStub) DecodeCallback(context.Context, channelcap.CallbackInput) (channelcap.CallbackEvent, error) {
	return stub.event, nil
}

func TestWeChatCallbackPostgreSQLDeduplicatesAndNeverRegressesTerminalResult(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	ctx := t.Context()
	digest := strings.Repeat("a", 64)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_notify_channel SET provider_code='callback_fake',status_code='enabled',configuration_generation=1,configuration_version=2,non_secret_configuration='{"app_id":"wx_test_app"}',credential_secret_ref='callback-handle',credential_updated_at=CURRENT_TIMESTAMP,callback_verified_generation=1,callback_verified_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND channel_code='wechat_official'`); err != nil {
		t.Fatal(err)
	}
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_notify_delivery_chain (tenant_id,chain_no,root_task_no,tail_task_no) VALUES (0,'nchain_callback','ntask_callback','ntask_callback')`); insertErr != nil {
			return insertErr
		}
		_, insertErr := tx.Exec(`INSERT INTO plugin_idc_notify_delivery_task (tenant_id,task_no,chain_no,event_code,topic_code,channel_code,locale_code,business_object_type,business_object_no,body_content,content_digest,status_code,result_class,external_boundary_crossed,provider_message_ref,version) VALUES (0,'ntask_callback','nchain_callback','support.ticket_updated','support','wechat_official','zh-CN','support_ticket','ticket_callback','body',?,'submitted','accepted',TRUE,'123456789',2)`, digest)
		return insertErr
	})
	if err != nil {
		t.Fatal(err)
	}
	service := &serviceImpl{database: database, adapters: map[string]adapterBinding{"callback_fake": {adapter: callbackAdapterStub{event: channelcap.CallbackEvent{ProviderMessageRef: "123456789", OccurredAt: time.Now().UTC(), ResultClass: channelcap.ResultDelivered, EvidenceDigest: strings.Repeat("b", 64)}}}}}
	input := EncryptedCallbackInput{Signature: "signature", Timestamp: "1", Nonce: "nonce", Encrypted: []byte("encrypted")}
	if err = service.AcceptWeChatCallback(ctx, input); err != nil {
		t.Fatal(err)
	}
	input.Encrypted = []byte("encrypted")
	if err = service.AcceptWeChatCallback(ctx, input); err != nil {
		t.Fatal(err)
	}
	statusValue, err := database.Model(deliveryTable).Ctx(ctx).Fields("status_code").Where("tenant_id", 0).Where("task_no", "ntask_callback").Value()
	if err != nil || statusValue.String() != string(DeliveryDelivered) {
		t.Fatalf("status=%q err=%v", statusValue.String(), err)
	}
	count, err := database.Model(wechatCallbackTable).Ctx(ctx).Where("tenant_id", 0).Count()
	if err != nil || count != 1 {
		t.Fatalf("callback count=%d err=%v", count, err)
	}
	service.adapters["callback_fake"] = adapterBinding{adapter: callbackAdapterStub{event: channelcap.CallbackEvent{ProviderMessageRef: "123456789", OccurredAt: time.Now().UTC(), ResultClass: channelcap.ResultInvalidRecipient, EvidenceDigest: strings.Repeat("c", 64)}}}
	input.Encrypted = []byte("another")
	if err = service.AcceptWeChatCallback(ctx, input); err != nil {
		t.Fatal(err)
	}
	statusValue, err = database.Model(deliveryTable).Ctx(ctx).Fields("status_code").Where("tenant_id", 0).Where("task_no", "ntask_callback").Value()
	if err != nil || statusValue.String() != string(DeliveryDelivered) {
		t.Fatalf("terminal status regressed: %q err=%v", statusValue.String(), err)
	}
}
