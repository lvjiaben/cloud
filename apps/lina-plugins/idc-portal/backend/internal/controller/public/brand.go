package public

import (
	"context"
	"lina-plugin-idc-portal/backend/internal/service/brand"
	"lina-plugin-idc-portal/backend/internal/service/siteconfiguration"
	"strings"

	"lina-plugin-idc-portal/backend/api/public/v1"
)

func (c *ControllerV1) BrandGet(ctx context.Context, _ *v1.BrandGetReq) (*v1.BrandGetRes, error) {
	settings, active, err := c.sites.Public(ctx)
	if err != nil {
		return nil, err
	}
	if !active {
		return &v1.BrandGetRes{}, nil
	}
	text := func(value string) brand.LocalizedText { return brand.LocalizedText{ENUS: value, ZHCN: value} }
	config := brand.Config{SiteName: text(settings.Common.SiteName), LogoURL: settings.Common.LogoURL, ContactEmail: settings.Common.ContactEmail, ContactPhone: settings.Common.ContactPhone, Footer: text(settings.Common.CompanyName), SEOTitle: text(settings.Common.SiteTitle), SEODescription: text(settings.Common.Description), PrimaryColor: "#1677ff", Navigation: []brand.NavItem{}, HomepageBlocks: []brand.HomepageBlock{}}
	get := func(key string) string { value, _ := settings.Config[key].(string); return value }
	if color := get("primarycolor"); color != "" {
		config.PrimaryColor = color
	}
	config.TemplateConfig = settings.Config
	config.TemplateEntry = settings.Entry
	config.TemplateRenderer = settings.Renderer
	config.HeroTitle = text(get("herotitle"))
	config.HeroDescription = text(get("herodescription"))
	config.HeroImage = get("heroimage")
	for _, line := range strings.Split(get("menutext"), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 && brand.SafePublicPath(strings.TrimSpace(parts[1])) {
			config.Navigation = append(config.Navigation, brand.NavItem{Label: text(strings.TrimSpace(parts[0])), Href: strings.TrimSpace(parts[1])})
		}
	}
	if settings.Navigation != nil {
		config.Navigation = navigationItems(siteconfiguration.ResolveNavigation(settings.Navigation.Header, settings.Pages))
		config.FooterNavigation = navigationItems(siteconfiguration.ResolveNavigation(settings.Navigation.Footer, settings.Pages))
	}
	return &v1.BrandGetRes{Brand: &config}, nil
}

// navigationItems projects resolved trees without exposing administrative identifiers.
func navigationItems(items []siteconfiguration.ResolvedNavigation) []brand.NavItem {
	result := make([]brand.NavItem, 0, len(items))
	for _, item := range items {
		result = append(result, brand.NavItem{Group: item.Group, Label: brand.LocalizedText{ENUS: item.NameEn, ZHCN: item.NameZh}, Href: item.Href, Customer: item.Customer, External: item.External, Children: navigationItems(item.Children)})
	}
	return result
}
