// This file validates atomic real capability collection, bounded refreshes and
// stale-channel rejection against an isolated PostgreSQL schema.
package providercapability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type collectorProvider struct {
	resourcecap.Provider
	snapshot *resourcecap.CapabilitySnapshot
	before   func()
	calls    int
}

func (p *collectorProvider) DiscoverCapabilities(context.Context) (*resourcecap.CapabilitySnapshot, error) {
	p.calls++
	if p.before != nil {
		p.before()
	}
	return p.snapshot, nil
}
func TestProviderCapabilityCollectionAtomicPostgreSQL(t *testing.T) {
	ctx, db := capabilityDatabase(t)
	provider := &collectorProvider{snapshot: completeCapabilities()}
	cleanups := 0
	collector, err := New(db, func(context.Context, string) (resourcecap.Provider, func(), error) {
		return provider, func() { cleanups++ }, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worked, err := collector.RunOne(ctx)
	if err != nil || !worked {
		t.Fatalf("collect worked=%v err=%v", worked, err)
	}
	assertCapabilityCount(t, ctx, db, "plugin_idc_resource_capability_snapshot", 1)
	assertCapabilityCount(t, ctx, db, "plugin_idc_resource_capability_snapshot_item", resourcecap.MaximumCapabilityCount)
	if worked, err = collector.RunOne(ctx); err != nil || worked || provider.calls != 1 || cleanups != 1 {
		t.Fatalf("fresh snapshot reread worked=%v calls=%d cleanups=%d err=%v", worked, provider.calls, cleanups, err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_capability_snapshot SET observed_at=CURRENT_TIMESTAMP-INTERVAL '6 minutes'`); err != nil {
		t.Fatal(err)
	}
	provider.snapshot.Capabilities = provider.snapshot.Capabilities[:1]
	if worked, err = collector.RunOne(ctx); err == nil || !worked {
		t.Fatal("partial matrix accepted")
	}
	assertCapabilityCount(t, ctx, db, "plugin_idc_resource_capability_snapshot_item", resourcecap.MaximumCapabilityCount)
	value, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_capability_snapshot WHERE status_code='complete'`)
	if err != nil || value.Int() != 1 {
		t.Fatal("failed discovery replaced complete matrix")
	}
}
func TestProviderCapabilityCollectionFencesChannelChangesPostgreSQL(t *testing.T) {
	ctx, db := capabilityDatabase(t)
	provider := &collectorProvider{snapshot: completeCapabilities(), before: func() {
		if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_channel SET version=version+1`); err != nil {
			t.Fatal(err)
		}
	}}
	collector, err := New(db, func(context.Context, string) (resourcecap.Provider, func(), error) { return provider, func() {}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = collector.RunOne(ctx); err == nil {
		t.Fatal("changed channel accepted stale capabilities")
	}
	assertCapabilityCount(t, ctx, db, "plugin_idc_resource_capability_snapshot", 0)
}
func TestProviderCapabilityCollectionSkipsDisabledPostgreSQL(t *testing.T) {
	ctx, db := capabilityDatabase(t)
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_channel SET status_code='disabled'`); err != nil {
		t.Fatal(err)
	}
	collector, err := New(db, func(context.Context, string) (resourcecap.Provider, func(), error) {
		t.Fatal("disabled channel constructed")
		return nil, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := collector.RunOne(ctx); err != nil || worked {
		t.Fatalf("disabled channel worked=%v err=%v", worked, err)
	}
}
func completeCapabilities() *resourcecap.CapabilitySnapshot {
	snapshot := &resourcecap.CapabilitySnapshot{AdapterVersion: "collector-test", ObservedAt: time.Now().UTC()}
	for _, code := range resourcecap.SupportedCapabilityCodes() {
		snapshot.Capabilities = append(snapshot.Capabilities, resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelSupported})
	}
	return snapshot
}
func assertCapabilityCount(t *testing.T, ctx context.Context, db gdb.DB, table string, want int) {
	t.Helper()
	n, err := db.GetValue(ctx, "SELECT count(*) FROM "+table)
	if err != nil || n.Int() != want {
		t.Fatalf("table %s count=%d want=%d err=%v", table, n.Int(), want, err)
	}
}
func capabilityDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := t.Context()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_RESOURCE_TEST_DSN for PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal("test database unavailable")
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_capability_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.WithoutCancel(ctx)
		if _, dropErr := admin.Exec(cleanup, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(cleanup); closeErr != nil {
			t.Error(closeErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal("test schema unavailable")
	}
	t.Cleanup(func() {
		if closeErr := db.Close(context.WithoutCancel(ctx)); closeErr != nil {
			t.Error(closeErr)
		}
	})
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/001-resource-foundation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_channel(channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code) VALUES('channel-capability-1','zjmfcloud-v1','idc-zjmfcloud','Capability Test','https://provider.invalid','enabled')`); err != nil {
		t.Fatal(err)
	}
	return ctx, db
}

func TestProviderCapabilityCollectionRollsBackWholeBatchPostgreSQL(t *testing.T) {
	ctx, db := capabilityDatabase(t)
	if _, err := db.Exec(ctx, `CREATE FUNCTION reject_capability_item() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated item failure'; END $$; CREATE TRIGGER reject_item BEFORE INSERT ON plugin_idc_resource_capability_snapshot_item FOR EACH ROW EXECUTE FUNCTION reject_capability_item()`); err != nil {
		t.Fatal(err)
	}
	collector, err := New(db, func(context.Context, string) (resourcecap.Provider, func(), error) {
		return &collectorProvider{snapshot: completeCapabilities()}, func() {}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = collector.RunOne(ctx); err == nil {
		t.Fatal("injected snapshot item failure hidden")
	}
	assertCapabilityCount(t, ctx, db, "plugin_idc_resource_capability_snapshot", 0)
	assertCapabilityCount(t, ctx, db, "plugin_idc_resource_capability_snapshot_item", 0)
}

func TestProviderCapabilityFailureRetriesSoonWithoutReplacingCompletePostgreSQL(t *testing.T) {
	ctx, db := capabilityDatabase(t)
	provider := &collectorProvider{snapshot: completeCapabilities()}
	collector, err := New(db, func(context.Context, string) (resourcecap.Provider, func(), error) { return provider, func() {}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := collector.RunOne(ctx); err != nil || !worked {
		t.Fatalf("initial capability matrix missing: %t %v", worked, err)
	}
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_capability_snapshot SET observed_at=CURRENT_TIMESTAMP-INTERVAL '6 minutes'`); err != nil {
		t.Fatal(err)
	}
	provider.snapshot = nil
	if worked, err := collector.RunOne(ctx); err == nil || !worked {
		t.Fatalf("failed discovery not recorded: %t %v", worked, err)
	}
	if worked, err := collector.RunOne(ctx); err != nil || worked {
		t.Fatalf("failed discovery retried without its short cooldown: %t %v", worked, err)
	}
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_capability_snapshot SET observed_at=CURRENT_TIMESTAMP-INTERVAL '31 seconds' WHERE status_code='failed'`); err != nil {
		t.Fatal(err)
	}
	provider.snapshot = completeCapabilities()
	if worked, err := collector.RunOne(ctx); err != nil || !worked {
		t.Fatalf("failed record blocked the repaired connector for five minutes: %t %v", worked, err)
	}
	count, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_capability_snapshot WHERE status_code='complete'`)
	if err != nil || count.Int() != 2 {
		t.Fatalf("old complete capabilities lost: count=%v %v", count, err)
	}
}

func TestProviderCapabilityDiscoveryPrioritizesUnseenChannelPostgreSQL(t *testing.T) {
	ctx, db := capabilityDatabase(t)
	calls := []string{}
	collector, err := New(db, func(_ context.Context, channel string) (resourcecap.Provider, func(), error) {
		calls = append(calls, channel)
		provider := &collectorProvider{}
		if channel == "channel-capability-2" {
			provider.snapshot = completeCapabilities()
		}
		return provider, func() {}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collector.RunOne(ctx); err == nil {
		t.Fatal("bad connector accepted")
	}
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_capability_snapshot SET observed_at=CURRENT_TIMESTAMP-INTERVAL '6 minutes'; INSERT INTO plugin_idc_resource_channel(channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code) VALUES('channel-capability-2','zjmfcloud-v1','idc-zjmfcloud','New healthy channel','https://provider.invalid','enabled')`); err != nil {
		t.Fatal(err)
	}
	calls = nil
	if worked, err := collector.RunOne(ctx); err != nil || !worked {
		t.Fatalf("old failing channel starved the unseen healthy channel: %t %v", worked, err)
	}
	if len(calls) != 1 || calls[0] != "channel-capability-2" {
		t.Fatalf("unseen channel not discovered first: %v", calls)
	}
}
