package brand

import (
	"path/filepath"
	"testing"
)

func validConfig(revision string) Config {
	return Config{
		Revision:       revision,
		SiteName:       LocalizedText{ENUS: "Example", ZHCN: "示例站点"},
		PrimaryColor:   "#176B4D",
		Navigation:     []NavItem{{Label: LocalizedText{ENUS: "Products", ZHCN: "商品"}, Href: "/products"}},
		HomepageBlocks: []HomepageBlock{{Title: LocalizedText{ENUS: "Simple delivery", ZHCN: "简单交付"}, Description: LocalizedText{ENUS: "Clear service choices", ZHCN: "清楚的服务选择"}, Href: "/products"}},
		Footer:         LocalizedText{ENUS: "All rights reserved", ZHCN: "版权所有"},
		SEOTitle:       LocalizedText{ENUS: "Example", ZHCN: "示例站点"},
		SEODescription: LocalizedText{ENUS: "Services", ZHCN: "服务"},
	}
}

func TestBrandRevisionLifecycle(t *testing.T) {
	svc, err := New(filepath.Join(t.TempDir(), "site-runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Save(validConfig("rev-1")); err != nil {
		t.Fatal(err)
	}
	if err = svc.Publish("rev-1"); err != nil {
		t.Fatal(err)
	}
	if _, published, err := svc.Current(); err != nil || !published {
		t.Fatalf("current brand: published=%v err=%v", published, err)
	}
	if _, err = svc.Save(validConfig("rev-2")); err != nil {
		t.Fatal(err)
	}
	if err = svc.Publish("rev-2"); err != nil {
		t.Fatal(err)
	}
	if err = svc.Rollback(); err != nil {
		t.Fatal(err)
	}
	current, published, err := svc.Current()
	if err != nil || !published || current.Revision != "rev-1" {
		t.Fatalf("rollback result: %#v published=%v err=%v", current, published, err)
	}
}

func TestBrandValidationRejectsUnsafeNavigationAndCode(t *testing.T) {
	config := validConfig("unsafe")
	config.Navigation[0].Href = "/admin"
	if err := Validate(config); err == nil {
		t.Fatal("expected reserved navigation path to be rejected")
	}
	config = validConfig("unsafe-code")
	config.Footer.ZHCN = "<script>alert(1)</script>"
	if err := Validate(config); err == nil {
		t.Fatal("expected code snippet to be rejected")
	}
	config = validConfig("unsafe-block")
	config.HomepageBlocks[0].Href = "/admin/settings"
	if err := Validate(config); err == nil {
		t.Fatal("expected reserved homepage block path to be rejected")
	}
}

func TestBrandValidationRequiresStructuredCollections(t *testing.T) {
	config := validConfig("missing-blocks")
	config.HomepageBlocks = nil
	if err := Validate(config); err == nil {
		t.Fatal("expected missing homepage blocks to be rejected")
	}
}
