package renewalsource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/renewalownercap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const renewalSourceDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

func TestResourceRenewalOwnerPostgreSQLFreezesTaskFacts(t *testing.T) {
	ctx, database := newRenewalSourceDatabase(t)
	seedRenewalOwnerRoute(t, ctx, database)
	service, err := New(database)
	if err != nil {
		t.Fatalf("construct renewal owner: %v", err)
	}
	request := renewalownercap.TaskRequest{
		Version: renewalownercap.ContractVersion, OwnerID: OwnerID,
		OperationNo: "operation-renewal-1", EntitlementNo: "ent_11111111111111111111111111111111",
		PriceSnapshotNo: "price-renewal-1", PluginID: "connector-test",
		PluginReleaseVersion: "1.0.0", ContractVersion: "v1",
		DeclarationDigest: strings.Repeat("b", 64), RenewalMethod: "server.renew",
		ExpectedEntitlementVersion: 2,
		OriginalExpiresAt:          time.Now().UTC().Add(24 * time.Hour),
		ProjectedExpiresAt:         time.Now().UTC().Add(31 * 24 * time.Hour),
	}
	parameters, parameterErr := taskParameters(request)
	if parameterErr != nil {
		t.Fatalf("encode renewal parameters: %v", parameterErr)
	}
	if validateErr := (resourcecap.OperationRequest{
		RequestID: taskRequestID(request.OperationNo), Capability: resourcecap.CapabilityRenew,
		ResourceRef: request.EntitlementNo, Parameters: parameters,
	}).Validate(); validateErr != nil {
		t.Fatalf("renewal Provider operation is invalid: %v parameters=%s", validateErr, parameters)
	}
	created, err := service.CreateTask(ctx, request)
	if err != nil || created == nil || created.Reused || created.Status != renewalownercap.TaskStatusPending {
		t.Fatalf("create Provider renewal task=%+v err=%v", created, err)
	}
	replayed, err := service.CreateTask(ctx, request)
	if err != nil || replayed == nil || !replayed.Reused || replayed.TaskNo != created.TaskNo {
		t.Fatalf("replay Provider renewal task=%+v err=%v", replayed, err)
	}
	changed := request
	changed.ProjectedExpiresAt = changed.ProjectedExpiresAt.Add(24 * time.Hour)
	if _, err = service.CreateTask(ctx, changed); !errors.Is(err, renewalownercap.ErrTaskConflict) {
		t.Fatalf("changed task replay returned %v", err)
	}
	forged := request
	forged.PluginID = "forged-connector"
	if _, err = service.CreateTask(ctx, forged); !errors.Is(err, renewalownercap.ErrOwnerUnavailable) {
		t.Fatalf("forged connector evidence returned %v", err)
	}
	wrongOwner := request
	wrongOwner.OwnerID = "other-owner"
	if _, err = service.CreateTask(ctx, wrongOwner); !errors.Is(err, renewalownercap.ErrOwnerUnavailable) {
		t.Fatalf("wrong renewal owner returned %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task
SET status_code='succeeded',finished_at=CURRENT_TIMESTAMP,failure_code='',result_unknown=FALSE,
    version=version+1,updated_at=CURRENT_TIMESTAMP WHERE task_no=?`, created.TaskNo); err != nil {
		t.Fatalf("complete Provider renewal task: %v", err)
	}
	succeeded, err := service.ReadTask(ctx, renewalownercap.TaskReadRequest{TaskNo: created.TaskNo, Request: request})
	if err != nil || succeeded == nil || succeeded.Status != renewalownercap.TaskStatusSucceeded {
		t.Fatalf("read succeeded Provider task=%+v err=%v", succeeded, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task
SET status_code='result_unknown',finished_at=NULL,failure_code='provider_timeout',result_unknown=TRUE,
    version=version+1,updated_at=CURRENT_TIMESTAMP WHERE task_no=?`, created.TaskNo); err != nil {
		t.Fatalf("mark Provider renewal task unknown: %v", err)
	}
	unknown, err := service.ReadTask(ctx, renewalownercap.TaskReadRequest{TaskNo: created.TaskNo, Request: request})
	if err != nil || unknown == nil || unknown.Status != renewalownercap.TaskStatusResultUnknown ||
		unknown.FailureCode != "provider_timeout" {
		t.Fatalf("read unknown Provider task=%+v err=%v", unknown, err)
	}
}

func seedRenewalOwnerRoute(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,
     provider_release_version,provider_release_digest,provider_declaration_digest,
     display_name,endpoint,status_code)
VALUES ('channel-renewal-1','provider-test','connector-test','dynamic','v1','1.0.0',
        repeat('a',64),repeat('b',64),'Renewal Test','https://provider.invalid','enabled');
INSERT INTO plugin_idc_resource_pool
    (pool_no,channel_no,display_name,resource_kind,status_code)
VALUES ('pool-renewal-1','channel-renewal-1','Renewal Pool','virtual_server','enabled');
INSERT INTO plugin_idc_resource_mapping
    (mapping_no,local_resource_no,channel_no,pool_no,external_resource_ref,status_code)
VALUES ('mapping-renewal-1','ent_11111111111111111111111111111111','channel-renewal-1',
        'pool-renewal-1','external-renewal-1','active');
INSERT INTO plugin_idc_resource_task
    (task_no,request_id_digest,capability_code,local_resource_no,channel_no,pool_no,status_code,
     sanitized_parameters,finished_at)
VALUES ('provision-renewal-1',repeat('c',64),'server.provision',
        'ent_11111111111111111111111111111111','channel-renewal-1','pool-renewal-1',
        'succeeded','{}'::jsonb,CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_resource_server_entitlement_projection
    (entitlement_no,source_intent_no,customer_no,local_resource_no,mapping_no,provider_task_no,
     interface_instance_no,provider_plugin_id,plugin_release_version,fulfillment_contract_version,
     server_contract_version,release_digest,declaration_digest,product_no,product_version_no,
     product_name_zh_cn,product_name_en_us)
VALUES ('ent_11111111111111111111111111111111','intent-renewal-1','customer-renewal-1',
        'ent_11111111111111111111111111111111','mapping-renewal-1','provision-renewal-1',
        'interface-renewal-1','connector-test','1.0.0','v1','v1',repeat('a',64),repeat('b',64),
        'product-renewal-1','version-renewal-1','续费测试服务器','Renewal test server');`); err != nil {
		t.Fatalf("seed resource renewal owner route: %v", err)
	}
}

func seedRenewalSourceRoute(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,
     provider_release_version,provider_release_digest,provider_declaration_digest,
     display_name,endpoint,status_code)
VALUES ('channel-renewal-1','provider-test','connector-test','dynamic','v1','1.0.0',
        repeat('a',64),repeat('b',64),'Renewal Test','https://provider.invalid','enabled');
INSERT INTO plugin_idc_resource_pool
    (pool_no,channel_no,display_name,resource_kind,status_code)
VALUES ('pool-renewal-1','channel-renewal-1','Renewal Pool','virtual_server','enabled');
INSERT INTO plugin_idc_resource_mapping
    (mapping_no,local_resource_no,channel_no,pool_no,external_resource_ref,status_code)
VALUES ('mapping-renewal-1','service-renewal-1','channel-renewal-1',
        'pool-renewal-1','external-renewal-1','active');`); err != nil {
		t.Fatalf("seed resource test route: %v", err)
	}
}

func newRenewalSourceDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(renewalSourceDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL resource renewal tests", renewalSourceDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create resource renewal admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate resource renewal schema: %v", err)
	}
	schema := "linapro_resource_renewal_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create resource renewal schema: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatalf("create scoped resource renewal database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		if strings.HasPrefix(schema, "linapro_resource_renewal_test_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		}
		_ = admin.Close(ctx)
	})
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve resource renewal migration root")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	paths, err := filepath.Glob(filepath.Join(root, "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve resource migrations: %v", err)
	}
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read resource migration %s: %v", filepath.Base(path), readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("apply resource migration %s: %v", filepath.Base(path), err)
		}
	}
	return ctx, database
}
