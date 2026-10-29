// Package siteconfiguration owns installed website selection and public content settings.
package siteconfiguration

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-portal/backend/cap/templatecap"
	"lina-plugin-idc-portal/backend/cap/websitecap"
)

// Common contains content shared by every installed website template.
type Common struct {
	HeaderLogoURL             string `json:"headerLogoUrl"`
	FooterLogoURL             string `json:"footerLogoUrl"`
	Copyright                 string `json:"copyright"`
	BusinessQQ                string `json:"businessQq"`
	CompanyAddress            string `json:"companyAddress"`
	EnterpriseWechatQRCodeURL string `json:"enterpriseWechatQrCodeUrl"`
	WechatOfficialQRCodeURL   string `json:"wechatOfficialQrCodeUrl"`
	TelecomLicenseNumber      string `json:"telecomLicenseNumber"`
	TelecomLicenseURL         string `json:"telecomLicenseUrl"`

	ICPNumber    string `json:"icpNumber"`
	ICPURL       string `json:"icpUrl"`
	PoliceNumber string `json:"policeNumber"`
	PoliceURL    string `json:"policeUrl"`
	SiteName     string `json:"siteName"`
	SiteTitle    string `json:"siteTitle"`
	CompanyName  string `json:"companyName"`
	ContactEmail string `json:"contactEmail"`
	ContactPhone string `json:"contactPhone"`
	LogoURL      string `json:"logoUrl"`
	Description  string `json:"description"`
}

// Settings stores a single active template and keeps each template's settings independently.
type Settings struct {
	Entry          *templatecap.Entry        `json:"entry,omitempty"`
	Pages          []websitecap.Page         `json:"pages,omitempty"`
	Navigation     *websitecap.Navigation    `json:"navigation,omitempty"`
	SelectedPlugin string                    `json:"selectedPlugin"`
	Common         Common                    `json:"common"`
	Config         map[string]any            `json:"config"`
	Configurations map[string]map[string]any `json:"configurations,omitempty"`
	Renderer       string                    `json:"renderer,omitempty"`
}

// Candidate exposes only safe schema and installed identity.
type Candidate struct {
	Entry    *templatecap.Entry                       `json:"entry,omitempty"`
	Pages    []websitecap.Page                        `json:"pages"`
	PluginID string                                   `json:"pluginId"`
	Name     string                                   `json:"name"`
	Schema   *capmodel.ConfigurationSchemaDeclaration `json:"schema"`
	Renderer string                                   `json:"renderer"`
}

// View is the editable website configuration snapshot.
type View struct {
	Settings
	Plugins []Candidate `json:"plugins"`
}

// Service reuses the host's authoritative extension registry and the portal storage root.
type Service interface {
	// Get lists installed templates and saved settings.
	Get(context.Context) (View, error)
	// Save validates and stores settings under provider lifecycle admission.
	Save(context.Context, Settings) error
	// Public excludes settings of unavailable providers.
	Public(context.Context) (Settings, bool, error)
}

// serviceImpl holds shared provider discovery and persistent storage.
type serviceImpl struct {
	extensions pluginhost.BusinessExtensionRegistrar
	path       string
	mu         sync.Mutex
}

// New constructs a service with explicit shared registry injection.
func New(extensions pluginhost.BusinessExtensionRegistrar, root string) (Service, error) {
	if extensions == nil {
		return nil, gerror.New("website plugin registry is unavailable")
	}
	if root == "" {
		root = os.Getenv("LINA_SITE_STORAGE")
	}
	if root == "" {
		root = ".site-runtime"
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &serviceImpl{extensions: extensions, path: filepath.Join(root, "website-configuration.json")}, nil
}

var _ Service = (*serviceImpl)(nil)
