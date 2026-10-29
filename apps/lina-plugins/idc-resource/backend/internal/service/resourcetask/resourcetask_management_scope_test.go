// This file proves entitlement task filtering happens before database paging and counting.
package resourcetask

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

type scopeTestAuthorizer struct{ permission string }

func (a scopeTestAuthorizer) Authorize(_ context.Context, request managementcap.AuthorizationRequest) (int, error) {
	if a.permission != "" && request.Permission != a.permission {
		return 0, errors.New("permission denied")
	}
	return 1, nil
}

func TestTaskListFiltersEntitlementBeforeCountAndPaging(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_RESOURCE_TEST_DSN for PostgreSQL task-scope verification")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "resource_task_scope_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(ctx); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	ddl := `CREATE TABLE plugin_idc_resource_channel (tenant_id bigint,channel_no text,display_name text);
 CREATE TABLE plugin_idc_resource_task (tenant_id bigint,task_no text,local_resource_no text,channel_no text,
 capability_code text DEFAULT 'server.start',status_code text DEFAULT 'succeeded',attempt_count int DEFAULT 1,
 max_attempts int DEFAULT 8,observation_attempt_count int DEFAULT 0,result_unknown boolean DEFAULT false,
 version bigint DEFAULT 1,next_observe_at timestamptz,created_at timestamptz,updated_at timestamptz);
 CREATE TABLE plugin_idc_resource_task_manual_takeover (tenant_id bigint,task_no text,takeover_no text,
 from_status_code text,task_version bigint,reason_code text,evidence_count int,assigned_user_id bigint,created_at timestamptz);
 CREATE TABLE plugin_idc_resource_server_entitlement_projection (tenant_id bigint,entitlement_no text,
 local_resource_no text,provider_task_no text);
 INSERT INTO plugin_idc_resource_channel VALUES (0,'channel_primary','Primary');
 INSERT INTO plugin_idc_resource_server_entitlement_projection VALUES (0,'SVC_A','resource_A','task_provision_A'),(0,'SVC_B','resource_B','task_provision_B');
 INSERT INTO plugin_idc_resource_task (tenant_id,task_no,local_resource_no,channel_no) VALUES
 (0,'task_1','resource_A','channel_primary'),(0,'task_2','SVC_A','channel_primary'),
 (0,'task_provision_A','provision_unmapped','channel_primary'),(0,'task_other','resource_B','channel_primary'),(0,'task_legacy','SVC_LEGACY','channel_primary');`
	if _, err = database.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	service := &managementService{store: &Store{database: database}, runtime: managementcap.Runtime{Authorizer: scopeTestAuthorizer{}}}
	first, err := service.List(ctx, TaskListInput{ServiceNo: "SVC_A", Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 3 || len(first.Items) != 2 || first.Items[0].TaskNo != "task_1" || first.Items[1].TaskNo != "task_2" {
		t.Fatalf("first page leaked or counted other products: %+v", first)
	}
	second, err := service.List(ctx, TaskListInput{ServiceNo: "SVC_A", Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != 3 || len(second.Items) != 1 || second.Items[0].TaskNo != "task_provision_A" {
		t.Fatalf("provision task was not linked: %+v", second)
	}
	missing, err := service.List(ctx, TaskListInput{ServiceNo: "SVC_MISSING", Page: 1, Limit: 2})
	if err != nil || missing.Total != 0 || len(missing.Items) != 0 {
		t.Fatalf("missing product returned global tasks: page=%+v err=%v", missing, err)
	}
	legacy, err := service.List(ctx, TaskListInput{ServiceNo: "SVC_LEGACY", Page: 1, Limit: 2})
	if err != nil || legacy.Total != 1 || len(legacy.Items) != 1 || legacy.Items[0].TaskNo != "task_legacy" {
		t.Fatalf("legacy local service tasks disappeared: %+v err=%v", legacy, err)
	}
	for _, scope := range []TaskScope{{ServiceNo: "SVC_B"}, {ChannelNo: "channel_other"}} {
		if _, err := service.Detail(ctx, "task_1", scope); err == nil {
			t.Fatalf("cross-context task detail accepted: %+v", scope)
		}
		if _, err := service.TakeOver(ctx, ManualTakeoverInput{Scope: scope, TaskNo: "task_1", ExpectedVersion: 1, ReasonCode: TakeoverReasonProviderResultUnresolved, EvidenceNos: []string{"evidence_11111111111111111111111111111111"}}); err == nil {
			t.Fatalf("cross-context task takeover accepted: %+v", scope)
		}
	}
	if err := service.ensureTaskScope(ctx, "task_1", TaskScope{ServiceNo: "SVC_A", ChannelNo: "channel_primary"}); err != nil {
		t.Fatalf("own scoped task refused: %v", err)
	}
	listOnly := &managementService{store: service.store, runtime: managementcap.Runtime{Authorizer: scopeTestAuthorizer{permission: permissionTaskList}}}
	if page, err := listOnly.List(ctx, TaskListInput{ServiceNo: "SVC_A", Limit: 2}); err != nil || page.Total != 3 {
		t.Fatalf("list-only role lost its allowed product list: %+v / %v", page, err)
	}
	if _, err := listOnly.Detail(ctx, "task_1", TaskScope{ServiceNo: "SVC_A"}); !bizerr.Is(err, CodeTaskManagementNotAccessible) {
		t.Fatalf("list-only role bypassed detail permission via a known task number: %v", err)
	}
	if _, err := listOnly.TakeOver(ctx, ManualTakeoverInput{TaskNo: "task_1", Scope: TaskScope{ServiceNo: "SVC_A"}, ExpectedVersion: 1, ReasonCode: TakeoverReasonProviderResultUnresolved, EvidenceNos: []string{"evidence_11111111111111111111111111111111"}}); !bizerr.Is(err, CodeTaskManagementNotAccessible) {
		t.Fatalf("list-only role bypassed takeover permission via a known task number: %v", err)
	}
	noAccess := &managementService{store: service.store, runtime: managementcap.Runtime{Authorizer: scopeTestAuthorizer{permission: "none"}}}
	if _, err := noAccess.List(ctx, TaskListInput{ServiceNo: "SVC_A", Limit: 2}); !bizerr.Is(err, CodeTaskManagementNotAccessible) {
		t.Fatalf("ungranted role read the product list directly: %v", err)
	}

}
