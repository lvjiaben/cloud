// This file guards public projection of host-normalized website schema keys.
package public

import (
	"context"
	"testing"

	"lina-plugin-idc-portal/backend/internal/service/siteconfiguration"
)

type publicWebsiteFixture struct {
	siteconfiguration.Service
	settings siteconfiguration.Settings
}

func (f publicWebsiteFixture) Public(context.Context) (siteconfiguration.Settings, bool, error) {
	return f.settings, true, nil
}

// TestBrandNormalizedTemplateFields verifies title, image and safe navigation survive projection.
func TestBrandNormalizedTemplateFields(t *testing.T) {
	c := &ControllerV1{sites: publicWebsiteFixture{settings: siteconfiguration.Settings{
		Renderer: "public-standard-v1",
		Common:   siteconfiguration.Common{SiteName: "Example", SiteTitle: "Example title"},
		Config:   map[string]any{"herotitle": "Template title", "herodescription": "Description", "heroimage": "/banner.png", "primarycolor": "#123456", "menutext": "Products|/products\nUnsafe|https://evil.example\nHelp|/help"},
	}}}
	res, err := c.BrandGet(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Brand == nil || res.Brand.HeroTitle.ZHCN != "Template title" || res.Brand.HeroImage != "/banner.png" || res.Brand.PrimaryColor != "#123456" || res.Brand.TemplateRenderer != "public-standard-v1" {
		t.Fatalf("lost normalized fields: %#v", res.Brand)
	}
	if len(res.Brand.Navigation) != 2 || res.Brand.Navigation[0].Href != "/products" || res.Brand.Navigation[1].Href != "/help" {
		t.Fatalf("unsafe or missing navigation: %#v", res.Brand.Navigation)
	}
}
