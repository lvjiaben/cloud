// This test verifies provider isolation and blockers through production aggregate SQL.
package businessimpact

import (
	"context"
	"errors"
	"os"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/pluginhost"
)

// The rollback-only regression uses real owner schema and no host mutations.
func TestBusinessImpactPostgreSQL(t *testing.T) {
	if _, err := NewLazy(nil); err == nil {
		t.Fatal("nil database factory accepted")
	}
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rollback := errors.New("rollback impact regression")
	handler, err := NewLazy(func() gdb.DB { return db })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = handler(ctx, pluginhost.PluginBusinessImpactInput{}); err == nil {
		t.Fatal("empty target accepted")
	}
	err = db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// First validate real owner schema; temporary shadows isolate aggregate fixtures.
		if _, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-unrelated"}); err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE TEMP TABLE plugin_idc_resource_channel AS SELECT * FROM public.plugin_idc_resource_channel WITH NO DATA; CREATE TEMP TABLE plugin_idc_resource_task AS SELECT * FROM public.plugin_idc_resource_task WITH NO DATA; CREATE TEMP TABLE plugin_idc_resource_principal_task AS SELECT * FROM public.plugin_idc_resource_principal_task WITH NO DATA; CREATE TEMP TABLE plugin_idc_resource_mapping AS SELECT * FROM public.plugin_idc_resource_mapping WITH NO DATA; CREATE TEMP TABLE plugin_idc_resource_channel_invocation_lease AS SELECT * FROM public.plugin_idc_resource_channel_invocation_lease WITH NO DATA;`); err != nil {
			return err
		}
		if _, err := tx.Exec(`ALTER TABLE plugin_idc_resource_channel ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_resource_task ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_resource_principal_task ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_resource_mapping ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_resource_channel_invocation_lease ALTER COLUMN tenant_id SET DEFAULT 0;`); err != nil {
			return err
		}
		if _, err := tx.Exec(fixtureSQL); err != nil {
			return err
		}
		items, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-test-provider"})
		if err != nil {
			return err
		}
		got := map[pluginhost.BusinessImpactCategory]pluginhost.PluginBusinessImpactItem{}
		for _, item := range items {
			got[item.Category] = item
		}
		for _, key := range []pluginhost.BusinessImpactCategory{pluginhost.BusinessImpactActiveServices, pluginhost.BusinessImpactPendingHistoricalActions} {
			if got[key].BlockingCount != 1 {
				t.Fatalf("missing blocker %s: %+v", key, got)
			}
		}
		if got[pluginhost.BusinessImpactInFlightTasks].BlockingCount != 2 {
			t.Fatalf("unresolved resource tasks missed: %+v", got)
		}
		if _, err := tx.Exec(`UPDATE plugin_idc_resource_task SET status_code='succeeded';
UPDATE plugin_idc_resource_principal_task SET status_code='succeeded';
UPDATE plugin_idc_resource_mapping SET status_code='released';
UPDATE plugin_idc_resource_channel_invocation_lease SET lease_until=CURRENT_TIMESTAMP-INTERVAL '1 second';`); err != nil {
			return err
		}
		drained, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-test-provider"})
		if err != nil {
			return err
		}
		for _, item := range drained {
			if item.BlockingCount != 0 || item.TotalCount == 0 {
				t.Fatalf("drained history lost or blocks: %+v", item)
			}
		}
		other, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-unrelated"})
		if err != nil {
			return err
		}
		for _, item := range other {
			if item.TotalCount != 0 || item.BlockingCount != 0 {
				t.Fatalf("foreign target leaked: %+v", item)
			}
		}
		return rollback
	})
	if err != nil && !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}

const fixtureSQL = `INSERT INTO plugin_idc_resource_channel(channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code) VALUES('impact-channel','impact-provider','impact-test-provider','测试','https://supplier.example','disabled');
INSERT INTO plugin_idc_resource_mapping(mapping_no,local_resource_no,channel_no,pool_no,status_code,external_resource_ref) VALUES('impact-mapping','impact-resource','impact-channel','impact-pool','active','service-1');
INSERT INTO plugin_idc_resource_channel_invocation_lease(channel_no,lease_token,lease_until) VALUES('impact-channel','ril_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',CURRENT_TIMESTAMP+INTERVAL '1 minute');
INSERT INTO plugin_idc_resource_task(channel_no,status_code) VALUES('impact-channel','running');
INSERT INTO plugin_idc_resource_principal_task(channel_no,status_code) VALUES('impact-channel','manual_review');`
