// These tests keep empty-menu presentation independent of paid service ownership.
package productdirectory

import "testing"

func TestConfiguredEmptyPagesPostgreSQL(t *testing.T) {
	_, service, _, writer := newDirectoryFixture(t)
	ctx := t.Context()
	ids := []string{"idc-resource", "yunsu-ssl"}
	menu, err := service.Save(ctx, SaveInput{NameZhCn: "物理机", NameEnUs: "Physical", PagePluginIDs: &ids})
	if err != nil {
		t.Fatal(err)
	}
	assertEmpty := func(expected int) {
		t.Helper()
		page, e := service.ListServices(ctx, menu.DirectoryNo, 1, 20)
		if e != nil || page.Total != 0 || len(page.Items) != 0 || len(page.Providers) != expected {
			t.Fatalf("page=%+v err=%v", page, e)
		}
		if expected > 0 && (page.Providers[0].PluginID != "idc-resource" || page.Providers[0].Total != 0) {
			t.Fatal(page.Providers)
		}
	}
	assertEmpty(2)
	input := SaveInput{DirectoryNo: menu.DirectoryNo, NameZhCn: menu.NameZhCn, NameEnUs: menu.NameEnUs, StatusCode: StatusEnabled, ExpectedVersion: menu.Version, SortOrder: 2}
	menu, err = service.Save(ctx, input)
	if err != nil || len(menu.PagePluginIDs) != 2 {
		t.Fatalf("omitted configuration lost: %+v %v", menu, err)
	}
	assertEmpty(2)
	input.ExpectedVersion = menu.Version
	dup := []string{"idc-resource", "idc-resource"}
	input.PagePluginIDs = &dup
	if _, err = service.Save(ctx, input); err == nil {
		t.Fatal("duplicate plugins accepted")
	}
	clear := []string{}
	input.PagePluginIDs = &clear
	writer.fail = true
	if _, err = service.Save(ctx, input); err == nil {
		t.Fatal("audit failure must roll back page selection")
	}
	assertEmpty(2)
	writer.fail = false
	menu, err = service.Save(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	assertEmpty(0)
	input.ExpectedVersion = menu.Version
	input.PagePluginIDs = &ids
	input.NavigationType = NavigationCustom
	input.TargetURL = "https://example.com/"
	if _, err = service.Save(ctx, input); err == nil {
		t.Fatal("non-product menu accepted content blocks")
	}
}
