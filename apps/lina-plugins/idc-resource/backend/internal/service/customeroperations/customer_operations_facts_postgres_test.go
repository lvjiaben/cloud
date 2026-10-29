// This file verifies customer-scoped local facts, bounded input, refresh failure,
// mapping changes, and shared leases against real resource migrations.
package customeroperations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverfacts"
)

type factsRuntime struct{ provider *factsProvider }

func (r factsRuntime) ConstructSession(context.Context, string) (resourcecap.Provider, func(), error) {
	return r.provider, func() {}, nil
}

type factsProvider struct {
	resourcecap.Provider
	calls        int
	failed       bool
	beforeReturn func()
}

func (p *factsProvider) QueryResource(_ context.Context, q resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
	p.calls++
	if p.beforeReturn != nil {
		p.beforeReturn()
	}
	if p.failed {
		return nil, errors.New("supplier disconnected")
	}
	return &resourcecap.ResourceSnapshot{ExternalResourceRef: q.ExternalResourceRef, State: resourcecap.ResourceStateRunning, ObservedAt: time.Now(), Data: json.RawMessage(`{"hostname":"cloud-demo","os_name":"Debian 12","addresses":["203.0.113.10"],"extra":"must-not-persist"}`)}, nil
}

func TestNativeFactsLocalBatchOwnershipPostgreSQL(t *testing.T) {
	ctx, db := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, db)
	seedDirectoryServer(t, ctx, db, "customer-a", "1", "active", "succeeded")
	seedDirectoryServer(t, ctx, db, "customer-b", "2", "active", "succeeded")
	seedDirectoryServer(t, ctx, db, "customer-a", "3", "released", "succeeded")
	provider := &factsProvider{}
	facts, err := serverfacts.New(db, factsRuntime{provider})
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"ent_" + strings.Repeat("1", 32), "ent_" + strings.Repeat("2", 32), "ent_" + strings.Repeat("3", 32), "ent_" + strings.Repeat("4", 32)}
	items, err := facts.Read(ctx, "customer-a", append(keys, keys[0]))
	if err != nil || len(items) != 1 {
		t.Fatalf("owned deduplicated batch: %+v %v", items, err)
	}
	if items[0].ObservedAt != nil || !items[0].Stale || items[0].LastObservedState != resourcecap.ResourceStateUnknown || items[0].AddressesConfirmed || len(items[0].Addresses) != 0 {
		t.Fatalf("unobserved facts: %+v", items[0])
	}
	for _, input := range [][]string{nil, {}, {"invalid"}} {
		items, err = facts.Read(ctx, "customer-a", input)
		if len(input) == 0 && (err != nil || len(items) != 0) {
			t.Fatalf("empty batch: %v", err)
		}
		if len(input) > 0 && !errors.Is(err, serverfacts.ErrInvalid) {
			t.Fatal("invalid ID accepted")
		}
	}
	duplicates := make([]string, 100)
	for i := range duplicates {
		duplicates[i] = keys[0]
	}
	items, err = facts.Read(ctx, "customer-a", duplicates)
	if err != nil || len(items) != 1 {
		t.Fatalf("100 duplicate inputs: %v", err)
	}
	if _, err = facts.Read(ctx, "customer-a", append(duplicates, keys[0])); !errors.Is(err, serverfacts.ErrInvalid) {
		t.Fatal("101 inputs accepted")
	}
	if provider.calls != 0 {
		t.Fatal("local read contacted supplier")
	}
}

func TestNativeFactsRefreshPreservesAndInvalidatesPostgreSQL(t *testing.T) {
	ctx, db := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, db)
	seedDirectoryServer(t, ctx, db, "customer-a", "1", "active", "succeeded")
	provider := &factsProvider{}
	facts, err := serverfacts.New(db, factsRuntime{provider})
	if err != nil {
		t.Fatal(err)
	}
	key := "ent_" + strings.Repeat("1", 32)
	if worked, err := facts.RunOne(ctx); err != nil || !worked {
		t.Fatalf("refresh: %v %v", worked, err)
	}
	items, err := facts.Read(ctx, "customer-a", []string{key})
	if err != nil || len(items) != 1 {
		t.Fatal(err)
	}
	if items[0].Hostname != "cloud-demo" || items[0].OSName != "Debian 12" || !items[0].AddressesConfirmed || items[0].Stale || len(items[0].Addresses) != 1 {
		t.Fatalf("projected facts: %+v", items[0])
	}
	raw, err := db.GetValue(ctx, `SELECT facts::text FROM plugin_idc_resource_server_runtime_fact WHERE mapping_no='mapping-1'`)
	if err != nil || strings.Contains(raw.String(), "must-not-persist") {
		t.Fatal("unapproved field persisted", err)
	}
	if worked, err := facts.RunOne(ctx); err != nil || worked || provider.calls != 1 {
		t.Fatalf("refresh interval not enforced: %v %v %d", worked, err, provider.calls)
	}
	forceFactsDue(t, ctx, db)
	provider.failed = true
	if worked, err := facts.RunOne(ctx); err != nil || !worked {
		t.Fatalf("failed observation handling: %v %v", worked, err)
	}
	items, err = facts.Read(ctx, "customer-a", []string{key})
	if err != nil || items[0].LastObservedState != resourcecap.ResourceStateRunning || items[0].Hostname != "cloud-demo" {
		t.Fatal("failure replaced historical facts", err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_server_runtime_fact SET observed_at=CURRENT_TIMESTAMP-INTERVAL '6 minutes'`); err != nil {
		t.Fatal(err)
	}
	items, err = facts.Read(ctx, "customer-a", []string{key})
	if err != nil || !items[0].Stale || items[0].ObservedAt == nil || items[0].LastObservedState != resourcecap.ResourceStateRunning {
		t.Fatal("stale facts lost historical meaning", err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_mapping SET version=version+1 WHERE mapping_no='mapping-1'`); err != nil {
		t.Fatal(err)
	}
	items, err = facts.Read(ctx, "customer-a", []string{key})
	if err != nil || items[0].ObservedAt != nil || items[0].Hostname != "" || items[0].LastObservedState != resourcecap.ResourceStateUnknown {
		t.Fatal("changed mapping reused old facts", err)
	}
}

func TestNativeFactsStaleLeaseCannotPublishPostgreSQL(t *testing.T) {
	ctx, db := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, db)
	seedDirectoryServer(t, ctx, db, "customer-a", "1", "active", "succeeded")
	provider := &factsProvider{beforeReturn: func() {
		if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_server_runtime_fact SET lease_version=lease_version+1,last_error_code='newer_lease' WHERE mapping_no='mapping-1'`); err != nil {
			t.Fatal(err)
		}
	}}
	facts, err := serverfacts.New(db, factsRuntime{provider})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = facts.RunOne(ctx); err != nil {
		t.Fatal(err)
	}
	var row struct {
		ObservedAt    *time.Time
		LastErrorCode string
	}
	if err = db.GetScan(ctx, &row, `SELECT observed_at,last_error_code FROM plugin_idc_resource_server_runtime_fact WHERE mapping_no='mapping-1'`); err != nil || row.ObservedAt != nil || row.LastErrorCode != "newer_lease" {
		t.Fatal("old lease overwrote current worker", err)
	}
}

func forceFactsDue(t *testing.T, ctx context.Context, db gdb.DB) {
	t.Helper()
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_server_runtime_fact SET next_refresh_at=CURRENT_TIMESTAMP-INTERVAL '1 second',lease_until=NULL`); err != nil {
		t.Fatal(err)
	}
}
