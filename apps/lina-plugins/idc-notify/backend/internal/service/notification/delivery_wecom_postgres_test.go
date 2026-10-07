package notification

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type wecomStaffDirectoryStub struct {
	active  bool
	version int64
	seen    []notificationcap.StaffDirectoryRequest
}

func (s *wecomStaffDirectoryStub) ResolveActiveBatch(
	_ context.Context,
	requests []notificationcap.StaffDirectoryRequest,
) ([]notificationcap.StaffDirectoryProjection, error) {
	s.seen = append(s.seen, requests...)
	result := make([]notificationcap.StaffDirectoryProjection, len(requests))
	for index := range result {
		result[index] = notificationcap.StaffDirectoryProjection{Active: s.active, CurrentVersion: s.version}
	}
	return result, nil
}

type wecomIdentityAccessorStub struct {
	mu    sync.Mutex
	calls int
}

func (s *wecomIdentityAccessorStub) Access(
	_ context.Context,
	_ secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	value := []byte("wecom-user-101")
	defer clear(value)
	return consume(value)
}

type wecomDeliveryAdapterStub struct {
	mu          sync.Mutex
	calls       int
	destination string
}

func (*wecomDeliveryAdapterStub) ValidateConfiguration(context.Context, map[string]string, map[string][]byte) error {
	return nil
}

func (*wecomDeliveryAdapterStub) ValidateDestination([]byte) error { return nil }

func (s *wecomDeliveryAdapterStub) Deliver(_ context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.destination = string(input.Destination)
	return channelcap.Result{Class: channelcap.ResultAccepted, CrossedExternalBorder: true,
		ProviderMessageRef: "wecom-message-1", EvidenceDigest: digestText("wecom-accepted", input.TaskNo)}, nil
}

func (*wecomDeliveryAdapterStub) Query(context.Context, channelcap.Query) (channelcap.Result, error) {
	return channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true}, nil
}

func TestWeComDeliveryRechecksCurrentStaffBeforeReleasingBindingPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	insertActiveWeComBinding(t, database, 101, 11)
	insertWeComDeliveryFixture(t, database, 101, "wecom-delivery-active", "sending", false)
	directory := &wecomStaffDirectoryStub{active: true, version: 11}
	accessor := &wecomIdentityAccessorStub{}
	adapter := &wecomDeliveryAdapterStub{}
	service := &serviceImpl{database: database,
		sources:  notificationcap.Runtime{StaffDirectory: directory},
		secrets:  map[string]SecretBinding{"wechat_work": {Manager: newChannelIntentSecretManager(), Accessor: accessor}},
		adapters: map[string]adapterBinding{providerWeCom: {adapter: adapter}},
	}
	ownerUserID := 101
	task := &deliveryRow{TaskNo: "wecom-delivery-active", ChannelCode: "wechat_work", TopicCode: "support",
		OwnerUserID: &ownerUserID, BodyContent: "A ticket needs your attention.", BusinessObjectVersion: 1,
		RetrySequence: 0, Version: 1, QueuedAt: time.Now().UTC()}
	if err := service.processClaimed(t.Context(), task); err != nil {
		t.Fatalf("deliver to a currently active bound staff member: %v", err)
	}
	if len(directory.seen) != 1 || directory.seen[0].HostUserID != ownerUserID || directory.seen[0].ExpectedVersion != 11 {
		t.Fatalf("staff authority was not rechecked with the binding fence: %+v", directory.seen)
	}
	if accessor.calls != 1 || adapter.calls != 1 || adapter.destination != "wecom-user-101" {
		t.Fatalf("unexpected destination release: accessor=%d adapter=%d destination=%q", accessor.calls, adapter.calls, adapter.destination)
	}
	assertWeComDeliveryState(t, database, task.TaskNo, string(DeliveryManualReview), string(channelcap.ResultAccepted), true)
}

func TestWeComDeliveryRejectsRevokedStaffBeforeSecretOrProviderBoundaryPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	insertActiveWeComBinding(t, database, 202, 22)
	insertWeComDeliveryFixture(t, database, 202, "wecom-delivery-revoked", "sending", false)
	directory := &wecomStaffDirectoryStub{active: false, version: 23}
	accessor := &wecomIdentityAccessorStub{}
	adapter := &wecomDeliveryAdapterStub{}
	service := &serviceImpl{database: database,
		sources:  notificationcap.Runtime{StaffDirectory: directory},
		secrets:  map[string]SecretBinding{"wechat_work": {Manager: newChannelIntentSecretManager(), Accessor: accessor}},
		adapters: map[string]adapterBinding{providerWeCom: {adapter: adapter}},
	}
	ownerUserID := 202
	task := &deliveryRow{TaskNo: "wecom-delivery-revoked", ChannelCode: "wechat_work", TopicCode: "support",
		OwnerUserID: &ownerUserID, RetrySequence: 0, Version: 1}
	if err := service.processClaimed(t.Context(), task); err != nil {
		t.Fatalf("reject an inactive staff recipient: %v", err)
	}
	if accessor.calls != 0 || adapter.calls != 0 {
		t.Fatalf("inactive staff crossed a secret or provider boundary: accessor=%d adapter=%d", accessor.calls, adapter.calls)
	}
	assertWeComDeliveryState(t, database, task.TaskNo, string(DeliveryFailed), string(channelcap.ResultInvalidRecipient), false)
	count, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_delivery_attempt
        WHERE tenant_id=0 AND task_no=?`, task.TaskNo)
	if err != nil || count.Int() != 0 {
		t.Fatalf("inactive staff created a provider attempt: count=%d err=%v", count.Int(), err)
	}
}

func insertActiveWeComBinding(t *testing.T, database gdb.DB, hostUserID int, staffVersion int64) {
	t.Helper()
	_, err := database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_wecom_employee_binding
        (tenant_id,binding_no,host_user_id,staff_version,corp_id,channel_generation,identity_digest,
         identity_secret_ref,identity_secret_updated_at,secret_operation_key,status_code,request_no,bound_at)
        VALUES (0,?,?,?,?,1,?, ?,CURRENT_TIMESTAMP,?,'active',?,CURRENT_TIMESTAMP)`,
		stableNumber("nwcb_", stringInt64(int64(hostUserID))), hostUserID, staffVersion, "ww_test_corp",
		digestText("wecom-identity", stringInt64(int64(hostUserID))),
		"sec_notify_wecom_identity_"+stringInt64(int64(hostUserID)),
		stableNumber("nwcsecop_", stringInt64(int64(hostUserID))),
		stableNumber("nwcreq_", stringInt64(int64(hostUserID))))
	if err != nil {
		t.Fatalf("insert active WeCom binding: %v", err)
	}
}

func assertWeComDeliveryState(
	t *testing.T,
	database gdb.DB,
	taskNo, status, resultClass string,
	crossed bool,
) {
	t.Helper()
	var state struct {
		StatusCode              string `orm:"status_code"`
		ResultClass             string `orm:"result_class"`
		ExternalBoundaryCrossed bool   `orm:"external_boundary_crossed"`
	}
	if err := database.Model(deliveryTable).Ctx(t.Context()).Where("tenant_id", 0).Where("task_no", taskNo).
		Fields("status_code,result_class,external_boundary_crossed").Scan(&state); err != nil {
		t.Fatalf("read WeCom delivery state: %v", err)
	}
	if state.StatusCode != status || state.ResultClass != resultClass || state.ExternalBoundaryCrossed != crossed {
		t.Fatalf("unexpected WeCom delivery state: %+v", state)
	}
}
