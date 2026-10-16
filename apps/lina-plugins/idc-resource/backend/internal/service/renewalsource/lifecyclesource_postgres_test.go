// This file verifies resource-owned lifecycle task intent and Provider parameters in PostgreSQL.

package renewalsource

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/lifecyclesource"
)

func TestResourceLifecycleSourcePostgreSQLFreezesTaskIntent(t *testing.T) {
	ctx, database := newRenewalSourceDatabase(t)
	seedRenewalSourceRoute(t, ctx, database)
	insertLifecycleCapabilitySnapshot(t, ctx, database, "capability-lifecycle-1", "supported", `[]`)
	service, err := lifecyclesource.New(database)
	if err != nil {
		t.Fatalf("construct lifecycle source: %v", err)
	}
	request := lifecycleresourcecap.TaskRequest{
		LifecycleTaskNo: "LCT_0123456789abcdef0123456789abcdef",
		ServiceNo:       "service-renewal-1", ChannelNo: "channel-renewal-1",
		PoolNo: "pool-renewal-1", Action: lifecycleresourcecap.ActionSuspend,
		ExpectedServiceVersion: 3, ExpectedExpiresAt: time.Now().UTC().Add(time.Hour),
		IdempotencyDigest: strings.Repeat("a", 64),
	}
	created, err := service.CreateTask(ctx, request)
	if err != nil || created == nil || created.Reused ||
		created.Status != lifecycleresourcecap.TaskStatusPending {
		t.Fatalf("create Provider lifecycle task=%+v err=%v", created, err)
	}
	assertLifecycleProviderParameters(t, ctx, database, created.ProviderTaskNo,
		`{"reason": "due"}`, request.LifecycleTaskNo)
	replayed, err := service.CreateTask(ctx, request)
	if err != nil || replayed == nil || !replayed.Reused ||
		replayed.ProviderTaskNo != created.ProviderTaskNo {
		t.Fatalf("replay Provider lifecycle task=%+v err=%v", replayed, err)
	}
	changed := request
	changed.Action = lifecycleresourcecap.ActionTerminate
	if _, err = service.CreateTask(ctx, changed); !errors.Is(err, lifecycleresourcecap.ErrTaskConflict) {
		t.Fatalf("changed lifecycle replay returned %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code = 'succeeded', finished_at = CURRENT_TIMESTAMP,
    failure_code = '', result_unknown = FALSE, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE task_no = ?`, created.ProviderTaskNo); err != nil {
		t.Fatalf("complete Provider lifecycle task: %v", err)
	}
	succeeded, err := service.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{
		ProviderTaskNo: created.ProviderTaskNo, Request: request,
	})
	if err != nil || succeeded == nil ||
		succeeded.Status != lifecycleresourcecap.TaskStatusSucceeded {
		t.Fatalf("read succeeded Provider lifecycle task=%+v err=%v", succeeded, err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code = 'failed', failure_code = 'UPSTREAM_REJECTED',
    failure_summary = 'upstream rejected the lifecycle request',
    result_unknown = FALSE, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE task_no = ?`, created.ProviderTaskNo); err != nil {
		t.Fatalf("fail Provider lifecycle task: %v", err)
	}
	failed, err := service.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{
		ProviderTaskNo: created.ProviderTaskNo, Request: request,
	})
	if err != nil || failed == nil ||
		failed.Status != lifecycleresourcecap.TaskStatusFailed ||
		failed.FailureCode != "upstream_rejected" {
		t.Fatalf("read failed Provider lifecycle task=%+v err=%v", failed, err)
	}
	for index, action := range []lifecycleresourcecap.Action{
		lifecycleresourcecap.ActionResume,
		lifecycleresourcecap.ActionTerminate,
	} {
		operation := request
		operation.LifecycleTaskNo = []string{
			"LCT_11111111111111111111111111111111",
			"LCT_22222222222222222222222222222222",
		}[index]
		operation.Action = action
		operation.IdempotencyDigest = strings.Repeat([]string{"c", "d"}[index], 64)
		projected, createErr := service.CreateTask(ctx, operation)
		if createErr != nil || projected == nil {
			t.Fatalf("create %s lifecycle task=%+v err=%v", action, projected, createErr)
		}
		assertLifecycleProviderParameters(t, ctx, database, projected.ProviderTaskNo,
			`{}`, operation.LifecycleTaskNo)
	}
	partial := request
	partial.LifecycleTaskNo = "LCT_44444444444444444444444444444444"
	partial.Action = lifecycleresourcecap.ActionTerminate
	partial.IdempotencyDigest = strings.Repeat("f", 64)
	partialTask, err := service.CreateTask(ctx, partial)
	if err != nil || partialTask == nil {
		t.Fatalf("create partial reclaim task=%+v err=%v", partialTask, err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code='result_unknown',failure_code='PARTIAL_RECLAIM',
    failure_summary='one normalized residual resource remains',result_unknown=TRUE,
    version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE task_no=?`, partialTask.ProviderTaskNo); err != nil {
		t.Fatalf("seed partial reclaim result: %v", err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task_evidence
    (evidence_no,task_no,source_kind,evidence_kind,reference,summary,observed_at)
VALUES ('evidence_44444444444444444444444444444444',?,'provider_task',
        'residualResource','volume_600100','volume|release_failed|provider_rejected',CURRENT_TIMESTAMP)`, partialTask.ProviderTaskNo); err != nil {
		t.Fatalf("seed partial reclaim projection: %v", err)
	}
	partialProjection, err := service.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{
		ProviderTaskNo: partialTask.ProviderTaskNo, Request: partial,
	})
	if err != nil || partialProjection == nil ||
		partialProjection.Status != lifecycleresourcecap.TaskStatusResultUnknown ||
		partialProjection.FailureCode != "partial_reclaim" ||
		len(partialProjection.ResidualResources) != 1 ||
		partialProjection.ResidualResources[0] != (lifecycleresourcecap.ResidualResource{
			ResourceKind: "volume", ResourceRef: "volume_600100",
			Status: "release_failed", ReasonCode: "provider_rejected",
		}) {
		t.Fatalf("read partial reclaim projection=%+v err=%v", partialProjection, err)
	}

	insertLifecycleCapabilitySnapshot(t, ctx, database,
		"capability-lifecycle-optional", "conditional", `[{
			"name":"reason","required":false,
			"allowedValues":["traffic","due","other"]
		}]`)
	optional := request
	optional.LifecycleTaskNo = "LCT_33333333333333333333333333333333"
	optional.IdempotencyDigest = strings.Repeat("e", 64)
	optionalCreated, err := service.CreateTask(ctx, optional)
	if err != nil || optionalCreated == nil {
		t.Fatalf("optional due-reason capability rejected task=%+v err=%v", optionalCreated, err)
	}
	assertLifecycleProviderParameters(t, ctx, database, optionalCreated.ProviderTaskNo,
		`{"reason":"due"}`, optional.LifecycleTaskNo)

	insertLifecycleCapabilitySnapshot(t, ctx, database,
		"capability-lifecycle-conditional", "conditional", `[{
			"name":"dangerous","required":true
		}]`)
	second := request
	second.LifecycleTaskNo = "LCT_abcdef0123456789abcdef0123456789"
	second.IdempotencyDigest = strings.Repeat("b", 64)
	if _, err = service.CreateTask(ctx, second); !errors.Is(err, lifecycleresourcecap.ErrCapabilityUnavailable) {
		t.Fatalf("conditional lifecycle capability returned %v", err)
	}
}

// TestEntitlementLifecycleOwnerRoutingPostgreSQL proves modern suspension and
// recovery use resource-owned customer bindings without a legacy service row.
func TestEntitlementLifecycleOwnerRoutingPostgreSQL(t *testing.T) {
	ctx, database := newRenewalSourceDatabase(t)
	seedRenewalOwnerRoute(t, ctx, database)
	insertLifecycleCapabilitySnapshot(t, ctx, database, "capability-entitlement-lifecycle", "supported", `[]`)
	service, err := lifecyclesource.New(database)
	if err != nil {
		t.Fatal(err)
	}
	request := lifecycleresourcecap.TaskRequest{
		LifecycleTaskNo: "LCT_0123456789abcdef0123456789abcdef",
		ServiceNo:       "ent_11111111111111111111111111111111", CustomerNo: "customer-renewal-1",
		Action: lifecycleresourcecap.ActionSuspend, ExpectedServiceVersion: 3,
		ExpectedExpiresAt: time.Now().UTC(), IdempotencyDigest: strings.Repeat("a", 64),
	}
	foreign := request
	foreign.CustomerNo = "another-customer"
	if _, err = service.CreateTask(ctx, foreign); !errors.Is(err, lifecycleresourcecap.ErrSourceUnavailable) {
		t.Fatalf("foreign customer created lifecycle task: %v", err)
	}
	forged := request
	forged.ChannelNo, forged.PoolNo = "channel-renewal-1", "pool-renewal-1"
	if _, err = service.CreateTask(ctx, forged); !errors.Is(err, lifecycleresourcecap.ErrSourceUnavailable) {
		t.Fatalf("caller-specified route was accepted: %v", err)
	}
	count, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_task WHERE capability_code<>'server.provision'`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("rejected ownership requests created tasks: %v %v", count, err)
	}
	created, err := service.CreateTask(ctx, request)
	if err != nil || created == nil || created.Status != lifecycleresourcecap.TaskStatusPending {
		t.Fatalf("entitlement suspend=%+v err=%v", created, err)
	}
	assertLifecycleProviderParameters(t, ctx, database, created.ProviderTaskNo, `{"reason":"due"}`, request.LifecycleTaskNo)
	replayed, err := service.CreateTask(ctx, request)
	if err != nil || replayed == nil || !replayed.Reused || replayed.ProviderTaskNo != created.ProviderTaskNo {
		t.Fatalf("entitlement replay=%+v err=%v", replayed, err)
	}
	if _, err = service.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{ProviderTaskNo: created.ProviderTaskNo, Request: foreign}); !errors.Is(err, lifecycleresourcecap.ErrSourceUnavailable) {
		t.Fatalf("foreign customer read lifecycle task: %v", err)
	}
	read, err := service.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{ProviderTaskNo: created.ProviderTaskNo, Request: request})
	if err != nil || read == nil || read.ProviderTaskNo != created.ProviderTaskNo {
		t.Fatalf("owned lifecycle read=%+v err=%v", read, err)
	}
	resume := request
	resume.LifecycleTaskNo = "LCT_22222222222222222222222222222222"
	resume.Action = lifecycleresourcecap.ActionResume
	resume.IdempotencyDigest = strings.Repeat("b", 64)
	restored, err := service.CreateTask(ctx, resume)
	if err != nil || restored == nil || restored.ProviderTaskNo == created.ProviderTaskNo {
		t.Fatalf("entitlement resume=%+v err=%v", restored, err)
	}
	assertLifecycleProviderParameters(t, ctx, database, restored.ProviderTaskNo, `{}`, resume.LifecycleTaskNo)
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_mapping SET status_code='released',released_at=CURRENT_TIMESTAMP WHERE mapping_no='mapping-renewal-1'`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{ProviderTaskNo: created.ProviderTaskNo, Request: request}); !errors.Is(err, lifecycleresourcecap.ErrSourceUnavailable) {
		t.Fatalf("released mapping still readable through old task: %v", err)
	}
}

func assertLifecycleProviderParameters(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	taskNo string,
	wantParameters string,
	wantLifecycleTaskNo string,
) {
	t.Helper()
	var row struct {
		Parameters      string `orm:"parameters"`
		LifecycleTaskNo string `orm:"lifecycle_task_no"`
	}
	if err := database.GetScan(ctx, &row, `
SELECT sanitized_parameters::TEXT AS parameters,
       owner_metadata->>'lifecycleTaskNo' AS lifecycle_task_no
FROM plugin_idc_resource_task
WHERE task_no = ?`, taskNo); err != nil {
		t.Fatalf("load Provider lifecycle parameters: %v", err)
	}
	var got any
	var want any
	if json.Unmarshal([]byte(row.Parameters), &got) != nil ||
		json.Unmarshal([]byte(wantParameters), &want) != nil ||
		!reflect.DeepEqual(got, want) || row.LifecycleTaskNo != wantLifecycleTaskNo {
		t.Fatalf("Provider parameters=%s owner task=%s want=%s/%s",
			row.Parameters, row.LifecycleTaskNo, wantParameters, wantLifecycleTaskNo)
	}
}

func insertLifecycleCapabilitySnapshot(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	snapshotNo string,
	support string,
	constraints string,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_capability_snapshot
    (snapshot_no, channel_no, adapter_version, status_code, item_count,
     observed_at, completed_at)
VALUES (?, 'channel-renewal-1', 'test-v1', 'complete', ?,
        CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		snapshotNo, resourcecap.MaximumCapabilityCount); err != nil {
		t.Fatalf("insert lifecycle capability snapshot: %v", err)
	}
	for _, capability := range resourcecap.SupportedCapabilityCodes() {
		itemSupport := "unsupported"
		itemConstraints := `[]`
		reason := "not used by lifecycle fixture"
		if capability == resourcecap.CapabilitySuspend ||
			capability == resourcecap.CapabilityResume ||
			capability == resourcecap.CapabilityTerminate {
			itemSupport = support
			itemConstraints = constraints
			if support != "unsupported" {
				reason = ""
			}
		}
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_capability_snapshot_item
    (snapshot_no, capability_code, support_code, constraints, unsupported_reason)
VALUES (?, ?, ?, CAST(? AS JSONB), ?)`, snapshotNo, capability,
			itemSupport, itemConstraints, reason); err != nil {
			t.Fatalf("insert lifecycle capability item %s: %v", capability, err)
		}
	}
}
