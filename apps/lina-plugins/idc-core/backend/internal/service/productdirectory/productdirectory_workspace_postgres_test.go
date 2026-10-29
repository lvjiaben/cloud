// This file checks literal search/count ownership and transactional customer notes
// against disposable PostgreSQL schemas; it never contacts a resource Provider.
package productdirectory

import (
	"lina-core/pkg/bizerr"
	"testing"
)

func TestWorkspaceSearchAndNoteIsolationPostgreSQL(t *testing.T) {
	db, service, access, writer := newDirectoryFixture(t)
	ctx := t.Context()
	note, err := service.GetNote(ctx, "ent-a")
	if err != nil || note.Revision != 1 || note.Remark != "" {
		t.Fatalf("initial note: %+v %v", note, err)
	}
	saved, err := service.SaveNote(ctx, CustomerNote{EntitlementNo: "ent-a", Remark: "生产_100%", Revision: 1})
	if err != nil || saved.Revision != 2 || writer.calls != 1 {
		t.Fatalf("save note: %+v %v", saved, err)
	}
	for _, term := range []string{"生产_100%", "ent-a", "冻结云"} {
		page, e := service.ListServices(ctx, AllDirectory, 1, 1, term)
		if e != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].EntitlementNo != "ent-a" || page.Items[0].CustomerRemark != "生产_100%" {
			t.Fatalf("search %q: %+v %v", term, page, e)
		}
	}
	empty, err := service.ListServices(ctx, AllDirectory, 1, 20, "_%")
	if err != nil || empty.Total != 0 {
		t.Fatalf("wildcards must be literal: %+v %v", empty, err)
	}
	if _, err = service.SaveNote(ctx, CustomerNote{EntitlementNo: "ent-a", Remark: "stale", Revision: 1}); !bizerr.Is(err, CodeNoteConflict) {
		t.Fatalf("stale note accepted: %v", err)
	}
	version, err := db.GetValue(ctx, `SELECT version FROM plugin_idc_core_service_entitlement WHERE entitlement_no='ent-a'`)
	if err != nil || version.Int64() != 7 {
		t.Fatal("note changed lifecycle version", err)
	}
	for _, target := range []string{"ent-b", "missing"} {
		if _, err = service.GetNote(ctx, target); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("foreign/missing read %s: %v", target, err)
		}
		if _, err = service.SaveNote(ctx, CustomerNote{EntitlementNo: target, Remark: "other", Revision: 1}); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("foreign/missing write %s: %v", target, err)
		}
	}
	writer.fail = true
	if _, err = service.SaveNote(ctx, CustomerNote{EntitlementNo: "ent-a", Remark: "rollback", Revision: 2}); err == nil {
		t.Fatal("audit failure accepted")
	}
	current, err := service.GetNote(ctx, "ent-a")
	if err != nil || current.Remark != "生产_100%" || current.Revision != 2 {
		t.Fatal("audit failure left partial note", current, err)
	}
	access.customer = "customer-b"
	foreign, err := service.ListServices(ctx, AllDirectory, 1, 20, "生产")
	if err != nil || foreign.Total != 0 {
		t.Fatal("search leaked another customer note", foreign, err)
	}
}

func TestWorkspacePriceOwnershipPostgreSQL(t *testing.T) {
	db, service, _, _ := newDirectoryFixture(t)
	ctx := t.Context()
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot VALUES(0,'price-a','customer-a',12345,'USD','month',1,3),(0,'price-b','customer-b',99999,'CNY','year',1,1); UPDATE plugin_idc_core_service_entitlement SET current_price_snapshot_no='price-a' WHERE entitlement_no='ent-a'; UPDATE plugin_idc_core_service_entitlement SET current_price_snapshot_no='price-b' WHERE entitlement_no='ent-c';`); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListServices(ctx, AllDirectory, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if item.EntitlementNo == "ent-a" {
			if item.CurrentPriceAmountMinor == nil || *item.CurrentPriceAmountMinor != "12345" || item.CurrencyCode != "USD" || item.ChargeCycles != 3 {
				t.Fatal("actual snapshot changed", item)
			}
		} else if item.CurrentPriceAmountMinor != nil {
			t.Fatal("cross-customer amount exposed", item)
		}
	}
}

func TestWorkspaceMultilineNotePostgreSQL(t *testing.T) {
	_, service, _, _ := newDirectoryFixture(t)
	note, err := service.SaveNote(t.Context(), CustomerNote{EntitlementNo: "ent-a", Remark: "  生产环境\r\n每晚备份  ", Revision: 1})
	if err != nil || note.Remark != "生产环境\n每晚备份" {
		t.Fatalf("textarea note was not preserved: %+v %v", note, err)
	}
	if _, err = service.SaveNote(t.Context(), CustomerNote{EntitlementNo: "ent-a", Remark: "invalid\x00", Revision: 2}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("control character accepted: %v", err)
	}
}

func TestWorkspaceCurrentPriceIsNotCashReceiptPostgreSQL(t *testing.T) {
	db, service, _, _ := newDirectoryFixture(t)
	ctx := t.Context()
	_, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot VALUES(0,'effective-upgrade','customer-a',8000,'USD','month',1,1); UPDATE plugin_idc_core_service_entitlement SET current_price_snapshot_no='effective-upgrade' WHERE entitlement_no='ent-a'`)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListServices(ctx, AllDirectory, 1, 20, "ent-a")
	if err != nil || len(page.Items) != 1 || page.Items[0].CurrentPriceAmountMinor == nil || *page.Items[0].CurrentPriceAmountMinor != "8000" {
		t.Fatalf("effective price projection: %+v %v", page, err)
	}
}
