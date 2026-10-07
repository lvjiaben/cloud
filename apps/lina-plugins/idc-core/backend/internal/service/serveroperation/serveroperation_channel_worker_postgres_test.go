package serveroperation

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type channelWorkerSource struct {
	task    *serveroperationresourcecap.Task
	request serveroperationresourcecap.TaskRequest
}

func (source *channelWorkerSource) CreateTask(
	_ context.Context,
	request serveroperationresourcecap.TaskRequest,
) (*serveroperationresourcecap.Task, error) {
	source.request = request
	value := *source.task
	return &value, nil
}

func (source *channelWorkerSource) ReadTask(
	_ context.Context,
	request serveroperationresourcecap.TaskReadRequest,
) (*serveroperationresourcecap.Task, error) {
	source.request = request.Request
	value := *source.task
	return &value, nil
}

func TestChannelWorkerPersistsDispatchAndUnknownResultEventsPostgreSQL(t *testing.T) {
	t.Parallel()
	dsn := strings.TrimSpace(os.Getenv(transferPostgresDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run channel worker PostgreSQL tests", transferPostgresDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(ctx) })
	schema := transferTestSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`) })
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, channelWorkerFixtureSQL); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	request := serveroperationresourcecap.TaskRequest{
		OperationNo: "COP_channel_resize_01", ServiceNo: "ENT_channel_01",
		ChannelNo: "resource-channel", PoolNo: "pool-01",
		Action: serveroperationresourcecap.ActionResize, ExpectedServiceVersion: 7,
		Parameters: []byte(`{"direction": "upgrade", "configurationSnapshotNo": "CFG_target"}`),
	}
	requestDigest, err := serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_service_entitlement
(tenant_id,entitlement_no,version,version_no,billing_rule_no,current_price_snapshot_no,updated_at)
VALUES(0,?,7,'VER_current','BILL_current','PRICE_current',?)`, request.ServiceNo, now); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_channel_service_quote
(tenant_id,quote_no,service_no,action_code,effective_at,service_owner_customer_no,
payer_customer_no,currency_code,direction_code,target_configuration_snapshot_no,target_version_no,
target_billing_rule_no,target_recurring_amount_minor,target_subtotal_minor,target_net_amount_minor,
target_tax_amount_minor,resource_reservation_no)
VALUES(0,'CQU_channel_01',?, 'resize',?,'CUS_owner','CUS_payer','CNY','upgrade',
'CFG_target','VER_target','BILL_target',12000,10000,10000,2000,NULL)`, request.ServiceNo, now); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation
(tenant_id,operation_no,quote_no,status_code,version,updated_at)
VALUES(0,?,'CQU_channel_01','ready',2,?)`, request.OperationNo, now); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_channel_service_operation
(tenant_id,operation_no,service_no,action_code,parameters,resource_channel_no,resource_pool_no,
status_code,request_digest,task_evidence_digest,expected_service_version,version,observed_at,updated_at)
VALUES(0,?,?,'resize',?::JSONB,?,?,'ready',?,'',7,1,?,?)`, request.OperationNo,
		request.ServiceNo, string(request.Parameters), request.ChannelNo, request.PoolNo,
		requestDigest, now, now); err != nil {
		t.Fatal(err)
	}

	source := &channelWorkerSource{task: &serveroperationresourcecap.Task{
		ProviderTaskNo: "TASK_channel_resize_01", Status: serveroperationresourcecap.TaskStatusResultUnknown,
		FailureCode: "provider.result_unknown", CapabilityEvidenceNo: "CAP_channel_01",
		CapabilityEvidenceDigest: strings.Repeat("a", 64), EvidenceDigest: strings.Repeat("b", 64),
		ObservedAt: now.Add(time.Second),
	}}
	worker := &workerImpl{database: database, source: source}
	selected, dispatched, reconciled, manual, failures, runErr :=
		worker.processChannelPricedOperations(ctx, 1)
	if runErr != nil || selected != 1 || dispatched != 1 || reconciled != 0 || manual != 1 || failures != 0 {
		t.Fatalf("unexpected batch selected=%d dispatched=%d reconciled=%d manual=%d failures=%d err=%v",
			selected, dispatched, reconciled, manual, failures, runErr)
	}
	if source.request.RequestDigest != requestDigest || source.request.OperationNo != request.OperationNo {
		t.Fatalf("provider request lost its immutable identity: %+v", source.request)
	}

	var projection *struct {
		ChannelStatus, BusinessStatus, ProviderTaskNo string
	}
	if err = database.GetScan(ctx, &projection, `SELECT channel.status_code AS channel_status,
business.status_code AS business_status,channel.provider_task_no
FROM plugin_idc_core_channel_service_operation channel
JOIN plugin_idc_core_business_operation business USING(tenant_id,operation_no)
WHERE channel.tenant_id=0 AND channel.operation_no=?`, request.OperationNo); err != nil {
		t.Fatal(err)
	}
	if projection == nil || projection.ChannelStatus != "result_unknown" ||
		projection.BusinessStatus != "manual_review" || projection.ProviderTaskNo != source.task.ProviderTaskNo {
		t.Fatalf("unexpected channel projection: %+v", projection)
	}
	var events []struct {
		Sequence int `orm:"event_sequence"`
		From     string
		To       string
		Reason   string
	}
	if err = database.GetScan(ctx, &events, `SELECT event_sequence,from_status_code AS "from",
to_status_code AS "to",reason_code AS reason
FROM plugin_idc_core_business_operation_event
WHERE tenant_id=0 AND operation_no=? ORDER BY event_sequence`, request.OperationNo); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[0].From != "ready" ||
		events[0].To != "executing" || events[0].Reason != "channel.operation.task_created" ||
		events[1].Sequence != 2 || events[1].From != "executing" ||
		events[1].To != "manual_review" || events[1].Reason != "channel.operation.provider_observed" {
		t.Fatalf("unexpected channel operation events: %+v", events)
	}
}

const channelWorkerFixtureSQL = `
CREATE TABLE plugin_idc_core_service_entitlement(
 tenant_id INT NOT NULL,entitlement_no VARCHAR(64) NOT NULL,version BIGINT NOT NULL,
 version_no VARCHAR(40) NOT NULL,billing_rule_no VARCHAR(40) NOT NULL,
 current_price_snapshot_no VARCHAR(40),updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(tenant_id,entitlement_no));
CREATE TABLE plugin_idc_core_channel_service_quote(
 tenant_id INT NOT NULL,quote_no VARCHAR(40) NOT NULL,service_no VARCHAR(64) NOT NULL,
 action_code VARCHAR(24) NOT NULL,effective_at TIMESTAMPTZ NOT NULL,
 service_owner_customer_no VARCHAR(40) NOT NULL,payer_customer_no VARCHAR(40) NOT NULL,
 currency_code CHAR(3) NOT NULL,direction_code VARCHAR(16) NOT NULL,
 target_configuration_snapshot_no VARCHAR(40) NOT NULL,target_version_no VARCHAR(40) NOT NULL,
 target_billing_rule_no VARCHAR(40) NOT NULL,target_recurring_amount_minor BIGINT NOT NULL,
 target_subtotal_minor BIGINT NOT NULL,target_net_amount_minor BIGINT NOT NULL,
 target_tax_amount_minor BIGINT NOT NULL,resource_reservation_no VARCHAR(64),
 PRIMARY KEY(tenant_id,quote_no));
CREATE TABLE plugin_idc_core_business_operation(
 tenant_id INT NOT NULL,operation_no VARCHAR(40) NOT NULL,quote_no VARCHAR(40) NOT NULL,
 status_code VARCHAR(24) NOT NULL,provider_task_no VARCHAR(64),execution_started_at TIMESTAMPTZ,
 last_error_code VARCHAR(128) NOT NULL DEFAULT '',version BIGINT NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,PRIMARY KEY(tenant_id,operation_no));
CREATE TABLE plugin_idc_core_channel_service_operation(
 id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT NOT NULL,operation_no VARCHAR(40) NOT NULL,service_no VARCHAR(64) NOT NULL,
 action_code VARCHAR(24) NOT NULL,parameters JSONB NOT NULL,resource_channel_no VARCHAR(64) NOT NULL,
 resource_pool_no VARCHAR(64) NOT NULL,provider_task_no VARCHAR(64),status_code VARCHAR(24) NOT NULL,
 request_digest CHAR(64) NOT NULL,task_evidence_digest CHAR(64) NOT NULL,
 expected_service_version BIGINT NOT NULL,version BIGINT NOT NULL,observed_at TIMESTAMPTZ NOT NULL,
 failure_code VARCHAR(128) NOT NULL DEFAULT '',output_bindings JSONB NOT NULL DEFAULT '[]',
 completed_at TIMESTAMPTZ,updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(tenant_id,operation_no));
CREATE TABLE plugin_idc_core_business_operation_event(
 tenant_id INT NOT NULL,event_no VARCHAR(40) NOT NULL,operation_no VARCHAR(40) NOT NULL,
 service_no VARCHAR(64),subject_type_code VARCHAR(24) NOT NULL,subject_no VARCHAR(64) NOT NULL,
 event_sequence INT NOT NULL,from_status_code VARCHAR(24) NOT NULL,to_status_code VARCHAR(24) NOT NULL,
 reason_code VARCHAR(128) NOT NULL,actor_type_code VARCHAR(24) NOT NULL,
 actor_reference VARCHAR(128) NOT NULL,evidence_digest CHAR(64) NOT NULL,
 content_digest CHAR(64) NOT NULL,created_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(tenant_id,event_no),UNIQUE(tenant_id,operation_no,event_sequence));`
