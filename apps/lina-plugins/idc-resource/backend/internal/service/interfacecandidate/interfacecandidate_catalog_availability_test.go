package interfacecandidate

import (
	"strings"
	"testing"
)

func TestCatalogAvailabilityRejectsInvalidAndUnboundReads(t *testing.T) {
	source := NewLazy()
	for _, ids := range [][]string{{"../1"}, make([]string, 101), {"pfi_" + strings.Repeat("1", 32)}} {
		if _, err := source.CatalogAvailability(t.Context(), ids); err == nil {
			t.Fatal("invalid or unbound catalog read was accepted")
		}
	}
	if result, err := source.CatalogAvailability(t.Context(), nil); err != nil || len(result) != 0 {
		t.Fatalf("empty authorized page should need no owner read: %v", err)
	}
}

func TestCatalogAvailabilityUsesResourceMethodsAndScopePostgreSQL(t *testing.T) {
	db := newSnapshotTestDatabase(t)
	ctx := t.Context()
	ids := make([]string, 6)
	for index := range ids {
		ids[index] = "pfi_" + strings.Repeat(string(rune('1'+index)), 32)
	}
	for index := range 5 {
		tenant := 0
		if index == 3 {
			tenant = 1
		}
		deleted := index == 4
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_resource_channel(tenant_id,channel_no,status_code,deleted_at) VALUES(?,?,'enabled',CASE WHEN ? THEN CURRENT_TIMESTAMP END)`, tenant, ids[index], deleted); err != nil {
			t.Fatal(err)
		}
		methods := []string{"catalog.products.list", "catalog.products.detail"}
		if index == 1 {
			methods = nil
		} else if index == 2 {
			methods = methods[:1]
		}
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding(tenant_id,interface_instance_no,channel_no,resource_authorized_methods,product_authorized_methods) VALUES(?,?,?,?::text[],ARRAY['provision','query'])`, tenant, ids[index], ids[index], "{"+strings.Join(methods, ",")+"}"); err != nil {
			t.Fatal(err)
		}
	}
	// No outbound executor is installed: this path can only read local evidence.
	source := &LazySource{deps: &dependencies{database: db}}
	result, err := source.CatalogAvailability(ctx, ids)
	if err != nil || len(result) != len(ids) || !result[ids[0]] {
		t.Fatalf("catalog methods were not projected: %v %v", result, err)
	}
	for _, id := range ids[1:] {
		if result[id] {
			t.Fatalf("missing method, foreign tenant, deleted or absent interface exposed: %s", id)
		}
	}
}
