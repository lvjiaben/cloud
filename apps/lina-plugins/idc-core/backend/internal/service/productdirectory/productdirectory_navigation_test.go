// This file verifies member destination types preserve safe, explicit targets.
package productdirectory

import (
	"context"
	"lina-core/pkg/bizerr"
	"os"
	"path/filepath"
	"testing"
)

func TestMemberNavigationTargetValidation(t *testing.T) {
	for _, input := range []SaveInput{
		{NameZhCn: "测试", NameEnUs: "Test", NavigationType: NavigationSystem, TargetURL: "//evil.example/path"},
		{NameZhCn: "测试", NameEnUs: "Test", NavigationType: NavigationSystem, TargetURL: "/account/notifications"},
		{NameZhCn: "测试", NameEnUs: "Test", NavigationType: NavigationPlugin, TargetURL: "/account/notifications/?legacy=1"},
		{NameZhCn: "测试", NameEnUs: "Test", NavigationType: NavigationCustom, TargetURL: "javascript:alert(1)"},
		{NameZhCn: "测试", NameEnUs: "Test", NavigationType: NavigationCustom, TargetURL: "https://example.com", Icon: "not an icon"},
	} {
		if _, err := prepareSave(input, true); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("unsafe target accepted: %#v %v", input, err)
		}
	}
}

func TestMemberNavigationTypesPostgreSQL(t *testing.T) {
	_, service, _, _ := newDirectoryFixture(t)
	ctx := context.Background()
	for _, input := range []SaveInput{
		{NameZhCn: "订单", NameEnUs: "Orders", NavigationType: NavigationSystem, TargetURL: "/account/orders", Icon: "lucide:list"},
		{NameZhCn: "帮助", NameEnUs: "Help", NavigationType: NavigationCustom, TargetURL: "https://example.com/help", Icon: "lucide:book-open"},
		{NameZhCn: "插件", NameEnUs: "Plugin", NavigationType: NavigationPlugin, TargetURL: "/account/plugins/example/help", Icon: "lucide:plug"},
	} {
		created, err := service.Save(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if created.NavigationType != input.NavigationType || created.TargetURL != input.TargetURL || created.Icon != input.Icon {
			t.Fatalf("destination lost: %#v", created)
		}
		if _, err = service.ListServices(ctx, created.DirectoryNo, 1, 20); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("non-product menu exposed service lookup: %v", err)
		}
	}
	listed, err := service.ListAccount(ctx, 1, 100, "")
	if err != nil || len(listed.Items) != 3 {
		t.Fatalf("customer destinations missing: %#v %v", listed, err)
	}
	for _, item := range listed.Items {
		if item.NavigationType == NavigationProduct || item.TargetURL == "" {
			t.Fatalf("incorrect destination: %#v", item)
		}
	}
}

func TestDefaultMemberNavigationSeedPostgreSQL(t *testing.T) {
	db := directoryDatabase(t)
	ctx := context.Background()
	count, err := db.Model("plugin_idc_core_product_directory").Ctx(ctx).Where("navigation_type", "system").Count()
	if err != nil || count != 8 {
		t.Fatalf("default editable system menus: count=%d err=%v", count, err)
	}
	if _, err := db.Exec(ctx, "UPDATE plugin_idc_core_product_directory SET status_code='disabled',name_zh_cn='我的订单' WHERE target_url='/account/orders'"); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "129-member-navigation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, string(source)); err != nil {
		t.Fatal(err)
	}
	row, err := db.Model("plugin_idc_core_product_directory").Ctx(ctx).Where("target_url", "/account/orders").One()
	if err != nil || row["status_code"].String() != "disabled" || row["name_zh_cn"].String() != "我的订单" {
		t.Fatalf("seed overwrote customization: %v %v", row, err)
	}
}

func TestProductNavigationFilterPostgreSQL(t *testing.T) {
	db, service, access, _ := newDirectoryFixture(t)
	ctx := context.Background()
	system, err := service.Save(ctx, SaveInput{NameZhCn: "订单", NameEnUs: "Orders", NavigationType: NavigationSystem, TargetURL: "/account/orders", SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Save(ctx, SaveInput{NameZhCn: "云服务器", NameEnUs: "Cloud servers", NavigationType: NavigationProduct, SortOrder: 2})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Save(ctx, SaveInput{NameZhCn: "域名", NameEnUs: "Domains", NavigationType: NavigationProduct, SortOrder: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Save(ctx, SaveInput{NameZhCn: "帮助", NameEnUs: "Help", NavigationType: NavigationCustom, TargetURL: "https://example.com/help", SortOrder: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Save(ctx, SaveInput{NameZhCn: "插件", NameEnUs: "Plugin", NavigationType: NavigationPlugin, TargetURL: "/account/plugins/example/help", SortOrder: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO plugin_idc_core_product_directory_membership(product_no,directory_no) VALUES('product-a',?),('product-c',?)", first.DirectoryNo, second.DirectoryNo); err != nil {
		t.Fatal(err)
	}
	all, err := service.ListAccount(ctx, 1, 100, "")
	if err != nil || all.Total != 5 || len(all.Items) != 5 {
		t.Fatalf("all navigation changed: %#v %v", all, err)
	}
	for index, want := range []string{first.DirectoryNo, second.DirectoryNo} {
		page, readErr := service.ListAccount(ctx, index+1, 1, NavigationProduct)
		if readErr != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].DirectoryNo != want || page.Items[0].ServiceCount != 1 {
			t.Fatalf("product filter paging and owned counts: %#v %v", page, readErr)
		}
	}
	onlySystem, err := service.ListAccount(ctx, 1, 1, NavigationSystem)
	if err != nil || onlySystem.Total != 1 || len(onlySystem.Items) != 1 || onlySystem.Items[0].DirectoryNo != system.DirectoryNo {
		t.Fatalf("explicit system filter: %#v %v", onlySystem, err)
	}
	if _, err = service.ListServices(ctx, system.DirectoryNo, 1, 20); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("system menu became a service filter: %v", err)
	}
	access.customer = "customer-b"
	page, err := service.ListAccount(ctx, 2, 1, NavigationProduct)
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].ServiceCount != 0 {
		t.Fatalf("other customer scope leaked: %#v %v", page, err)
	}
	if _, err = service.ListAccount(ctx, 1, 100, NavigationType("invalid")); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("unknown type accepted: %v", err)
	}
	access.deny = true
	if _, err = service.ListAccount(ctx, 1, 100, NavigationProduct); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("filter bypassed authentication: %v", err)
	}
}

// TestRetiredNotificationNavigationMigrationPostgreSQL checks upgrade replay and child preservation.
func TestRetiredNotificationNavigationMigrationPostgreSQL(t *testing.T) {
	database := directoryDatabase(t)
	if _, err := database.Exec(t.Context(), `INSERT INTO plugin_idc_core_product_directory(directory_no,name_zh_cn,name_en_us,navigation_type,target_url) VALUES ('pdr_retired_notifications','旧通知','Old notifications','system',?),('pdr_retired_child','保留菜单','Keep child','system','/account/profile')`, "/account/notifications?legacy=1"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_core_product_directory SET parent_directory_no='pdr_retired_notifications' WHERE directory_no='pdr_retired_child'`); err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "156-remove-customer-notifications.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(t.Context(), string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT count(*) FROM plugin_idc_core_product_directory WHERE directory_no='pdr_retired_notifications'`, 0},
		{`SELECT count(*) FROM plugin_idc_core_product_directory WHERE directory_no='pdr_retired_child' AND parent_directory_no='' AND target_url='/account/profile'`, 1},
	} {
		count, err := database.GetValue(t.Context(), check.query)
		if err != nil || count.Int() != check.want {
			t.Fatalf("navigation migration: %v want %d error %v", count, check.want, err)
		}
	}
}
