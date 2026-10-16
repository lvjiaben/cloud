// This file verifies bounded configuration pages and the one-call usage seam.

package providerconfiguration

import (
	"context"
	"fmt"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	providerspi "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
)

// testUsageReader supplies explicit zero facts to unrelated configuration tests.
type testUsageReader struct{}

func (testUsageReader) BatchGet(_ context.Context, channels []string) ([]resourceinterfaceprojectioncap.Usage, error) {
	result := make([]resourceinterfaceprojectioncap.Usage, len(channels))
	for index, channel := range channels {
		result[index].ChannelNo = channel
	}
	return result, nil
}

// recordingUsageReader proves that one page invokes exactly one bounded batch.
type recordingUsageReader struct{ calls [][]string }

func (reader *recordingUsageReader) BatchGet(_ context.Context, channels []string) ([]resourceinterfaceprojectioncap.Usage, error) {
	reader.calls = append(reader.calls, append([]string(nil), channels...))
	result := make([]resourceinterfaceprojectioncap.Usage, len(channels))
	for index, channel := range channels {
		result[index] = resourceinterfaceprojectioncap.Usage{ChannelNo: channel, ActiveInstances: 3, TotalInstances: 5, ConfiguredProducts: 2}
	}
	return result, nil
}

func TestProviderConfigurationPagesPostgreSQL(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	_, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel
	(channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code,deleted_at)
	SELECT 'CHN_page_'||n, 'server-api','server-connector','Interface '||n,'https://supplier.example',CASE WHEN n=121 THEN 'deleted' ELSE 'disabled' END, CASE WHEN n=121 THEN CURRENT_TIMESTAMP END
	FROM generate_series(1,121) n`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_pool
	(pool_no,channel_no,display_name,resource_kind,status_code)
	SELECT 'POL_page_'||n, 'CHN_page_121','Pool '||n,'virtual_server','disabled' FROM generate_series(1,521) n`)
	if err != nil {
		t.Fatal(err)
	}
	usage := &recordingUsageReader{}
	service, err := New(database, managementcap.Runtime{Authorizer: providerConfigurationAuthorizer{}, Audit: providerConfigurationAudit{}},
		[]providerspi.Declaration{{Key: resourcecap.ProviderKey{PluginID: "server-connector", ProviderID: "server-api", ContractVersion: resourcecap.ContractVersion}, ProvisionParameters: testProviderConfigurationParameters}}, usage, testConfigurationProjection(t, database))
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{10, 20, 50, 100} {
		before := len(usage.calls)
		page, loadErr := service.List(ctx, ListInput{ChannelPage: 2, ChannelPageSize: size, PoolPage: 2, PoolPageSize: size})
		if loadErr != nil {
			t.Fatalf("size=%d: %v", size, loadErr)
		}
		expectedChannels := min(size, 121-size)
		if page.ChannelTotal != 121 || page.PoolTotal != 521 || len(page.Channels) != expectedChannels || len(page.Pools) != size ||
			page.Channels[0].ChannelNo != fmt.Sprintf("CHN_page_%d", size+1) || page.Pools[0].ChannelName != "Interface 121" ||
			page.Channels[0].Usage == nil || page.Channels[0].Usage.TotalInstances != 5 ||
			len(usage.calls) != before+1 || len(usage.calls[before]) != expectedChannels {
			t.Fatalf("incorrect page=%+v usage=%+v", page, usage.calls)
		}
	}
	page, err := service.List(ctx, ListInput{ChannelKeyword: "Interface 121", AvailableChannelsOnly: true})
	if err != nil || page.ChannelTotal != 0 || len(page.Channels) != 0 || page.PoolTotal != 521 {
		t.Fatalf("selector=%+v err=%v", page, err)
	}
	page, err = service.List(ctx, ListInput{ChannelKeyword: "Interface 121"})
	if err != nil || page.ChannelTotal != 1 || len(page.Channels) != 1 {
		t.Fatalf("search=%+v err=%v", page, err)
	}
	page, err = service.List(ctx, ListInput{ChannelPage: 99, PoolPage: 99})
	if err != nil || page.ChannelTotal != 121 || page.PoolTotal != 521 || len(page.Channels) != 0 || len(page.Pools) != 0 {
		t.Fatalf("empty=%+v err=%v", page, err)
	}
}

func TestNormalizeConfigurationPages(t *testing.T) {
	input := ListInput{}
	if err := normalizeListInput(&input); err != nil || input.ChannelPage != 1 || input.ChannelPageSize != 20 || input.PoolPageSize != 20 {
		t.Fatalf("defaults=%+v err=%v", input, err)
	}
	for _, size := range []int{10, 20, 50, 100} {
		input = ListInput{ChannelPageSize: size, PoolPageSize: size}
		if err := normalizeListInput(&input); err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
	}
	for _, input := range []ListInput{{ChannelPage: -1}, {PoolPage: 1000001}, {ChannelPageSize: 101}, {PoolPageSize: 1}} {
		if normalizeListInput(&input) == nil {
			t.Fatalf("accepted invalid page %+v", input)
		}
	}
}

// testProjectionSink keeps unrelated configuration tests explicit and stateless.
type testProjectionSink struct{}

func (testProjectionSink) Upsert(context.Context, resourceinterfaceprojectioncap.Projection) error {
	return nil
}
func (testProjectionSink) RefreshUnused(ctx context.Context, _ resourceinterfaceprojectioncap.Projection, apply func(context.Context) error) error {
	return apply(ctx)
}

func testConfigurationProjection(t *testing.T, database gdb.DB) *interfaceprojection.Worker {
	t.Helper()
	worker, err := interfaceprojection.NewWorker(database, testProjectionSink{})
	if err != nil {
		t.Fatal(err)
	}
	return worker
}
