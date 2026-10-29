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
		Common:   siteconfiguration.Common{SiteName: "Example", SiteTitle: "Example title", CompanyName: "Example Ltd", LogoURL: "/brand.png", ICPNumber: "ICP 123", ICPURL: "https://records.example/icp", PoliceNumber: "Police 456", PoliceURL: "https://records.example/police"},
		Config:   map[string]any{"herotitle": "Template title", "herodescription": "Description", "heroimage": "/banner.png", "primarycolor": "#123456", "menutext": "Products|/products\nUnsafe|https://evil.example\nHelp|/help"},
	}}}
	res, err := c.BrandGet(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Brand == nil || res.Brand.HeroTitle.ZHCN != "Template title" || res.Brand.HeroImage != "/banner.png" || res.Brand.PrimaryColor != "#123456" || res.Brand.TemplateRenderer != "public-standard-v1" {
		t.Fatalf("lost normalized fields: %#v", res.Brand)
	}
	if res.Brand.CompanyName != "Example Ltd" || res.Brand.Footer.ZHCN != "Example Ltd" || res.Brand.LogoURL != "/brand.png" || res.Brand.ICPNumber != "ICP 123" || res.Brand.ICPURL != "https://records.example/icp" || res.Brand.PoliceNumber != "Police 456" || res.Brand.PoliceURL != "https://records.example/police" {
		t.Fatal("shared brand or registration projection lost")
	}
	if len(res.Brand.Navigation) != 2 || res.Brand.Navigation[0].Href != "/products" || res.Brand.Navigation[1].Href != "/help" {
		t.Fatalf("unsafe or missing navigation: %#v", res.Brand.Navigation)
	}
}

// TestBrandLegacyLogoFallback keeps existing single-logo consumers while allowing independent header/footer images.
func TestBrandLegacyLogoFallback(t *testing.T) {
	for _, logos := range [][2]string{{"", ""}, {"/header.png", ""}, {"", "/footer.png"}} {
		controller := &ControllerV1{sites: publicWebsiteFixture{settings: siteconfiguration.Settings{Common: siteconfiguration.Common{LogoURL: "/legacy.png", HeaderLogoURL: logos[0], FooterLogoURL: logos[1]}}}}
		result, err := controller.BrandGet(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		expectedHeader, expectedFooter := logos[0], logos[1]
		if expectedHeader == "" {
			expectedHeader = "/legacy.png"
		}
		if expectedFooter == "" {
			expectedFooter = "/legacy.png"
		}
		if result.Brand.LogoURL != "/legacy.png" || result.Brand.HeaderLogoURL != expectedHeader || result.Brand.FooterLogoURL != expectedFooter {
			t.Fatalf("legacy logo or independent override lost: %+v", result.Brand)
		}
	}
}
