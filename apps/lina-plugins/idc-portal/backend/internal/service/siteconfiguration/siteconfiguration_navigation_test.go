// This file verifies navigation destinations, parent safety and template changes.
package siteconfiguration

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-portal/backend/cap/websitecap"
	"testing"
)

// TestNavigationValidationAndProjection covers forged routes, cycles and account links.
func TestNavigationValidationAndProjection(t *testing.T) {
	pages := []websitecap.Page{{ID: "home", NameZh: "首页", NameEn: "Home", Path: "/"}}
	metadata := &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"pages": pages}}
	root := websitecap.NavigationItem{ID: "home-link", PageID: "home", NameZh: "首页", NameEn: "Home"}
	login := websitecap.NavigationItem{ID: "login", ParentID: "home-link", PageID: "customer-login", NameZh: "登录", NameEn: "Login", SortOrder: 1}
	external := websitecap.NavigationItem{ID: "docs", URL: "https://example.test/docs", NameZh: "文档", NameEn: "Docs"}
	nav := &websitecap.Navigation{Header: []websitecap.NavigationItem{root, login}, Footer: []websitecap.NavigationItem{external}}
	if err := validateNavigation(nav, metadata); err != nil {
		t.Fatal(err)
	}
	resolved := ResolveNavigation(nav.Header, pages)
	if len(resolved) != 1 || len(resolved[0].Children) != 1 || resolved[0].Children[0].Href != "/auth/login" || !resolved[0].Children[0].Customer {
		t.Fatalf("incorrect account destination: %#v", resolved)
	}
	if items := ResolveNavigation(nav.Header, nil); len(items) != 0 {
		t.Fatal("unavailable parent page still exposed")
	}
	for _, change := range []func(*websitecap.Navigation){
		func(n *websitecap.Navigation) { n.Header[0].PageID = "forged" },
		func(n *websitecap.Navigation) { n.Header[0].ParentID = "login" },
		func(n *websitecap.Navigation) { n.Header[0].NameZh = "" },
		func(n *websitecap.Navigation) { n.Header[1].ID = n.Header[0].ID },
		func(n *websitecap.Navigation) { n.Footer[0].URL = "javascript:alert(1)" },
		func(n *websitecap.Navigation) { n.Footer[0].URL = "https://user:pass@example.test" },
	} {
		invalid := &websitecap.Navigation{Header: []websitecap.NavigationItem{root, login}, Footer: []websitecap.NavigationItem{external}}
		change(invalid)
		if err := validateNavigation(invalid, metadata); err == nil {
			t.Fatalf("invalid navigation accepted: %#v", invalid)
		}
	}
}

// TestGroupNavigation preserves destination-free parents and hides empty groups.
func TestGroupNavigation(t *testing.T) {
	pages := []websitecap.Page{{ID: "products", NameZh: "商品", NameEn: "Products", Path: "/products.html"}}
	metadata := &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"pages": pages}}
	group := websitecap.NavigationItem{ID: "group", Group: true, NameZh: "产品", NameEn: "Products"}
	child := websitecap.NavigationItem{ID: "child", ParentID: "group", PageID: "products", NameZh: "列表", NameEn: "List"}
	nav := &websitecap.Navigation{Header: []websitecap.NavigationItem{group, child}}
	if err := validateNavigation(nav, metadata); err != nil {
		t.Fatal(err)
	}
	resolved := ResolveNavigation(nav.Header, pages)
	if len(resolved) != 1 || !resolved[0].Group || resolved[0].Href != "" || len(resolved[0].Children) != 1 || resolved[0].Children[0].Href != "/products.html" {
		t.Fatalf("invalid group projection: %#v", resolved)
	}
	if len(ResolveNavigation([]websitecap.NavigationItem{group}, pages)) != 0 {
		t.Fatal("empty group exposed")
	}
	nav.Header[0].PageID = "products"
	if validateNavigation(nav, metadata) == nil {
		t.Fatal("group with destination accepted")
	}
}
