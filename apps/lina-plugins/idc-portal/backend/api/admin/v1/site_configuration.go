// This file declares documented website selection and content configuration DTOs.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-portal/backend/cap/websitecap"
)

// WebsiteCommon contains public details shared by all templates.
type WebsiteCommon struct {
	HeaderLogoURL             string `json:"headerLogoUrl" dc:"Header logo image URL" eg:"/header-logo.png"`
	FooterLogoURL             string `json:"footerLogoUrl" dc:"Footer logo image URL" eg:"/footer-logo.png"`
	Copyright                 string `json:"copyright" dc:"Copyright text" eg:"© Example Cloud"`
	BusinessQQ                string `json:"businessQq" dc:"Business QQ" eg:"12345678"`
	CompanyAddress            string `json:"companyAddress" dc:"Company address" eg:"Example business park"`
	EnterpriseWechatQRCodeURL string `json:"enterpriseWechatQrCodeUrl" dc:"Enterprise WeChat QR code image URL" eg:"/enterprise-wechat.png"`
	WechatOfficialQRCodeURL   string `json:"wechatOfficialQrCodeUrl" dc:"WeChat official account QR code image URL" eg:"/wechat-official.png"`
	TelecomLicenseNumber      string `json:"telecomLicenseNumber" dc:"Value-added telecom license number" eg:"Example license"`
	TelecomLicenseURL         string `json:"telecomLicenseUrl" dc:"Value-added telecom license lookup URL" eg:"https://records.example/telecom"`

	ICPNumber    string `json:"icpNumber" dc:"Optional Website ICP registration number" eg:"Example registration"`
	ICPURL       string `json:"icpUrl" dc:"Optional Website ICP registration link" eg:"https://records.example/lookup"`
	PoliceNumber string `json:"policeNumber" dc:"Optional Public security registration number" eg:"Example registration"`
	PoliceURL    string `json:"policeUrl" dc:"Optional Public security registration link" eg:"https://records.example/lookup"`
	SiteName     string `json:"siteName" dc:"Website name" eg:"My Cloud"`
	SiteTitle    string `json:"siteTitle" dc:"Website title" eg:"Cloud services"`
	CompanyName  string `json:"companyName" dc:"Company name" eg:"Cloud Ltd"`
	ContactEmail string `json:"contactEmail" dc:"Contact email" eg:"help@example.com"`
	ContactPhone string `json:"contactPhone" dc:"Contact phone" eg:"4001234567"`
	LogoURL      string `json:"logoUrl" dc:"Logo image URL" eg:"/logo.png"`
	Description  string `json:"description" dc:"Website description" eg:"Reliable cloud services"`
}

// WebsiteCandidate describes an installed and enabled template.
type WebsiteCandidate struct {
	Pages    []websitecap.Page                        `json:"pages" dc:"Public pages declared by this installed template" eg:"[]"`
	PluginID string                                   `json:"pluginId" dc:"Website plugin identifier" eg:"website-default"`
	Name     string                                   `json:"name" dc:"Website plugin name" eg:"Standard website"`
	Schema   *capmodel.ConfigurationSchemaDeclaration `json:"schema" dc:"Plugin-specific configuration fields" eg:"{}"`
	Renderer string                                   `json:"renderer" dc:"Website renderer" eg:"public-standard-v1"`
}

// SiteConfigurationGetReq lists the editable website settings.
type SiteConfigurationGetReq struct {
	g.Meta `path:"/admin/site/configuration" method:"get" tags:"Website" summary:"Get website configuration" dc:"List installed enabled website templates and saved settings" permission:"idc:admin:portal:brand"`
}

// SiteConfigurationGetRes returns the current settings and available templates.
type SiteConfigurationGetRes struct {
	Navigation     *websitecap.Navigation    `json:"navigation,omitempty" dc:"Merchant header and footer navigation" eg:"{}"`
	SelectedPlugin string                    `json:"selectedPlugin" dc:"Selected website plugin" eg:"website-default"`
	Common         WebsiteCommon             `json:"common" dc:"Shared website details" eg:"{}"`
	Config         map[string]any            `json:"config" dc:"Selected template settings" eg:"{}"`
	Configurations map[string]map[string]any `json:"configurations,omitempty" dc:"Saved settings for each template" eg:"{}"`
	Renderer       string                    `json:"renderer,omitempty" dc:"Website renderer" eg:"public-standard-v1"`
	Plugins        []WebsiteCandidate        `json:"plugins" dc:"Installed enabled website templates" eg:"[]"`
}

// SiteConfigurationSaveReq selects a template and saves its public content.
type SiteConfigurationSaveReq struct {
	Navigation     *websitecap.Navigation `json:"navigation,omitempty" dc:"Header and footer trees; up to 100 items each, maximum three levels" eg:"{}"`
	g.Meta         `path:"/admin/site/configuration" method:"put" tags:"Website" summary:"Save website configuration" dc:"Save shared details and the selected installed template settings" permission:"idc:admin:portal:brand"`
	SelectedPlugin string         `json:"selectedPlugin" dc:"Selected website plugin" eg:"website-default"`
	Common         WebsiteCommon  `json:"common" dc:"Shared website details" eg:"{}"`
	Config         map[string]any `json:"config" dc:"Selected template settings" eg:"{}"`
}

// SiteConfigurationSaveRes confirms the save.
type SiteConfigurationSaveRes struct {
	Saved bool `json:"saved" dc:"Settings were saved" eg:"true"`
}
