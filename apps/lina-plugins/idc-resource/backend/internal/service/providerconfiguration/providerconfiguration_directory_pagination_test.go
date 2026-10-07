// This file checks resource directory pages against real owner migrations.

package providerconfiguration

import (
	"context"
	"fmt"
	"testing"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/internal/service/providercapacity"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
	"lina-plugin-idc-resource/backend/internal/service/resourceoverview"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// directoryAuthorizer deliberately records the exact permission before any query.
type directoryAuthorizer struct {
	allowed     bool
	permissions []string
}

func (actor *directoryAuthorizer) Authorize(_ context.Context, request managementcap.AuthorizationRequest) (int, error) {
	actor.permissions = append(actor.permissions, request.Permission)
	if actor.allowed {
		return 42, nil
	}
	return 0, nil
}

func TestResourceDirectoriesNumericPagesPostgreSQL(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	for _, statement := range []string{
		`INSERT INTO plugin_idc_resource_channel (channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code)
SELECT 'CHN_directory_'||lpad(n::text,4,'0'),'server-api','server-connector','Interface '||n,'https://supplier.example','disabled' FROM generate_series(1,121) n`,
		`INSERT INTO plugin_idc_resource_provider_capacity_observation (observation_no,channel_no,capacity_kind,status_code,observed_at,completed_at)
SELECT 'pco_'||lpad(n::text,32,'0'),'CHN_directory_0001',CASE WHEN n<=101 THEN 'region' ELSE 'stock' END,'complete',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP FROM generate_series(1,121) n`,
		`INSERT INTO plugin_idc_resource_task (task_no,request_id_digest,capability_code,channel_no)
SELECT 'task_'||lpad(n::text,32,'0'),lpad(n::text,64,'0'),'server.query',CASE WHEN n<=101 THEN 'CHN_directory_0001' ELSE 'CHN_directory_0002' END FROM generate_series(1,121) n`,
		`INSERT INTO plugin_idc_resource_pool(pool_no,channel_no,display_name,resource_kind)
SELECT 'POL_directory_'||lpad(n::text,4,'0'),'CHN_directory_0001','Pool '||lpad(n::text,4,'0'),'virtual_server' FROM generate_series(1,521) n`,
		`INSERT INTO plugin_idc_resource_financial_snapshot(snapshot_no,channel_no,snapshot_kind,external_resource_ref,currency_code,amount,observed_at)
SELECT 'FIN_directory_'||lpad(n::text,4,'0'),'CHN_directory_0001','cost','resource_'||n,'CNY',1,CURRENT_TIMESTAMP FROM generate_series(1,521) n`,
		`INSERT INTO plugin_idc_resource_financial_snapshot(snapshot_no,channel_no,snapshot_kind,external_resource_ref,currency_code,amount,observed_at)
VALUES('FIN_directory_replacement','CHN_directory_0001','cost','resource_1','CNY',2,CURRENT_TIMESTAMP)`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	actor := &directoryAuthorizer{allowed: true}
	runtime := managementcap.Runtime{Authorizer: actor, Audit: providerConfigurationAudit{}}
	health, err := providerhealth.NewManagement(database, runtime)
	if err != nil {
		t.Fatal(err)
	}
	capacity, err := providercapacity.NewManagement(database, runtime)
	if err != nil {
		t.Fatal(err)
	}
	store, err := resourcetask.NewStore(database)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := resourcetask.NewManagement(store, runtime)
	if err != nil {
		t.Fatal(err)
	}
	overview, err := resourceoverview.New(database, runtime)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{10, 20, 50, 100} {
		healthPage, loadErr := health.List(ctx, providerhealth.HealthListInput{Page: 2, Limit: size})
		if loadErr != nil || healthPage.Total != 121 || len(healthPage.Items) != min(size, 121-size) ||
			healthPage.Items[0].ChannelNo != fmt.Sprintf("CHN_directory_%04d", size+1) {
			t.Fatalf("health size %d: %+v / %v", size, healthPage, loadErr)
		}
		capacityPage, loadErr := capacity.ListOperations(ctx, providercapacity.OperationListInput{Page: 2, Limit: size, Kind: "region"})
		if loadErr != nil || capacityPage.Total != 101 || len(capacityPage.Items) != min(size, 101-size) {
			t.Fatalf("capacity size %d: %+v / %v", size, capacityPage, loadErr)
		}
		taskPage, loadErr := tasks.List(ctx, resourcetask.TaskListInput{Page: 2, Limit: size, ChannelNo: "CHN_directory_0001"})
		if loadErr != nil || taskPage.Total != 101 || len(taskPage.Items) != min(size, 101-size) ||
			taskPage.Items[0].TaskNo != fmt.Sprintf("task_%032d", size+1) {
			t.Fatalf("tasks size %d: %+v / %v", size, taskPage, loadErr)
		}
		summary, loadErr := overview.Get(ctx, resourceoverview.ListInput{PoolPage: 2, PoolPageSize: size, FinancePage: 3, FinancePageSize: size})
		if loadErr != nil || summary.PoolTotal != 521 || summary.FinanceTotal != 521 || len(summary.Pools) != size || len(summary.Finance) != size ||
			summary.Pools[0].PoolNo != fmt.Sprintf("POL_directory_%04d", size+1) {
			t.Fatalf("overview size %d: %+v / %v", size, summary, loadErr)
		}
	}
	// Empty tail pages keep the same exact totals rather than reporting zero records.
	tail, err := tasks.List(ctx, resourcetask.TaskListInput{Page: 99, Limit: 10})
	if err != nil || tail.Total != 121 || len(tail.Items) != 0 {
		t.Fatalf("tail %+v / %v", tail, err)
	}
	empty, err := overview.Get(ctx, resourceoverview.ListInput{PoolPage: 99, FinancePage: 99})
	if err != nil || empty.PoolTotal != 521 || empty.FinanceTotal != 521 || len(empty.Pools) != 0 || len(empty.Finance) != 0 {
		t.Fatalf("empty %+v / %v", empty, err)
	}
	actor.allowed = false
	if page, denial := health.List(ctx, providerhealth.HealthListInput{Page: 1, Limit: 10}); denial == nil || page != nil {
		t.Fatal("health permission denial leaked data")
	}
	if page, denial := capacity.ListOperations(ctx, providercapacity.OperationListInput{Page: 1, Limit: 10}); denial == nil || page != nil {
		t.Fatal("capacity permission denial leaked data")
	}
	if page, denial := tasks.List(ctx, resourcetask.TaskListInput{Page: 1, Limit: 10}); denial == nil || page != nil {
		t.Fatal("task permission denial leaked data")
	}
	if page, denial := overview.Get(ctx, resourceoverview.ListInput{}); denial == nil || page != nil {
		t.Fatal("overview permission denial leaked data")
	}
}
