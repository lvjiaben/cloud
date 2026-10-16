// Package brand defines the public website projection and navigation path boundary.
package brand

import (
	"lina-plugin-idc-portal/backend/cap/templatecap"
	"strings"
)

// LocalizedText is the bilingual text used by the public website contract.
type LocalizedText struct {
	ENUS string `json:"enUS"`
	ZHCN string `json:"zhCN"`
}

// Config is the public projection of the selected template and shared website configuration.
type Config struct {
	TemplateConfig   map[string]any     `json:"templateConfig,omitempty"`
	TemplateEntry    *templatecap.Entry `json:"templateEntry,omitempty"`
	TemplateRenderer string             `json:"templateRenderer"`
	HeroTitle        LocalizedText      `json:"heroTitle"`
	HeroDescription  LocalizedText      `json:"heroDescription"`
	HeroImage        string             `json:"heroImage"`
	Revision         string             `json:"revision"`
	SiteName         LocalizedText      `json:"siteName"`
	LogoURL          string             `json:"logoUrl,omitempty"`
	PrimaryColor     string             `json:"primaryColor"`
	ContactEmail     string             `json:"contactEmail,omitempty"`
	ContactPhone     string             `json:"contactPhone,omitempty"`
	FooterNavigation []NavItem          `json:"footerNavigation"`
	Navigation       []NavItem          `json:"navigation"`
	HomepageBlocks   []HomepageBlock    `json:"homepageBlocks"`
	Footer           LocalizedText      `json:"footer"`
	TermsURL         string             `json:"termsUrl,omitempty"`
	SEOTitle         LocalizedText      `json:"seoTitle"`
	SEODescription   LocalizedText      `json:"seoDescription"`
}

// HomepageBlock is a small, code-free content block rendered on the public home page.
// Links are optional; configuration is validated by the owning template service.
type HomepageBlock struct {
	Title       LocalizedText `json:"title"`
	Description LocalizedText `json:"description"`
	Href        string        `json:"href,omitempty"`
}

// NavItem is one safe public navigation link.
type NavItem struct {
	Group    bool          `json:"group,omitempty"`
	Customer bool          `json:"customer,omitempty"`
	External bool          `json:"external,omitempty"`
	Children []NavItem     `json:"children,omitempty"`
	Label    LocalizedText `json:"label"`
	Href     string        `json:"href"`
}

func safePublicPath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.HasPrefix(value, "/admin") && !strings.HasPrefix(value, "/api") && !strings.HasPrefix(value, "/x") && !strings.HasPrefix(value, "/account") && !strings.HasPrefix(value, "/auth") && !strings.HasPrefix(value, "/cart") && !strings.HasPrefix(value, "/checkout")
}

// SafePublicPath restricts legacy template links to public website routes.
func SafePublicPath(value string) bool { return safePublicPath(value) }
