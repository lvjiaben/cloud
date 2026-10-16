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
	listed, err := service.ListAccount(ctx, 1, 100)
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
