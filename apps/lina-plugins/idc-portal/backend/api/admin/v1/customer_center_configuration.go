// This file declares documented customer center selection and content configuration DTOs.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-portal/backend/cap/customertemplatecap"
)

// CustomerCenterCommon contains public details shared by all templates.
type CustomerCenterCommon struct {
	SiteName     string `json:"siteName" dc:"CustomerCenter name" eg:"My Cloud"`
	SiteTitle    string `json:"siteTitle" dc:"CustomerCenter title" eg:"Cloud services"`
	CompanyName  string `json:"companyName" dc:"Company name" eg:"Cloud Ltd"`
	ContactEmail string `json:"contactEmail" dc:"Contact email" eg:"help@example.com"`
	ContactPhone string `json:"contactPhone" dc:"Contact phone" eg:"4001234567"`
	LogoURL      string `json:"logoUrl" dc:"Logo image URL" eg:"/logo.png"`
	Description  string `json:"description" dc:"CustomerCenter description" eg:"Reliable cloud services"`
}

// CustomerCenterCandidate describes an installed and enabled template.
type CustomerCenterCandidate struct {
	Pages    []customertemplatecap.Page               `json:"pages" dc:"Public pages declared by this installed template" eg:"[]"`
	PluginID string                                   `json:"pluginId" dc:"CustomerCenter plugin identifier" eg:"customer-default"`
	Name     string                                   `json:"name" dc:"CustomerCenter plugin name" eg:"Standard customer center"`
	Schema   *capmodel.ConfigurationSchemaDeclaration `json:"schema" dc:"Plugin-specific configuration fields" eg:"{}"`
	Renderer string                                   `json:"renderer" dc:"CustomerCenter renderer" eg:"customer-standard-v1"`
}

// CustomerCenterConfigurationGetReq lists the editable customer center settings.
type CustomerCenterConfigurationGetReq struct {
	g.Meta `path:"/admin/customer-center/configuration" method:"get" tags:"CustomerCenter" summary:"Get customer center configuration" dc:"List installed enabled customer center templates and saved settings" permission:"idc:admin:portal:customer-template"`
}

// CustomerCenterConfigurationGetRes returns the current settings and available templates.
type CustomerCenterConfigurationGetRes struct {
	SelectedPlugin string                    `json:"selectedPlugin" dc:"Selected customer center plugin" eg:"customer-default"`
	Common         CustomerCenterCommon      `json:"common" dc:"Shared customer center details" eg:"{}"`
	Config         map[string]any            `json:"config" dc:"Selected template settings" eg:"{}"`
	Configurations map[string]map[string]any `json:"configurations,omitempty" dc:"Saved settings for each template" eg:"{}"`
	Renderer       string                    `json:"renderer,omitempty" dc:"CustomerCenter renderer" eg:"customer-standard-v1"`
	Plugins        []CustomerCenterCandidate `json:"plugins" dc:"Installed enabled customer center templates" eg:"[]"`
}

// CustomerCenterConfigurationSaveReq selects a template and saves its public content.
type CustomerCenterConfigurationSaveReq struct {
	g.Meta         `path:"/admin/customer-center/configuration" method:"put" tags:"CustomerCenter" summary:"Save customer center configuration" dc:"Save shared details and the selected installed template settings" permission:"idc:admin:portal:customer-template"`
	SelectedPlugin string               `json:"selectedPlugin" dc:"Selected customer center plugin" eg:"customer-default"`
	Common         CustomerCenterCommon `json:"common" dc:"Shared customer center details" eg:"{}"`
	Config         map[string]any       `json:"config" dc:"Selected template settings" eg:"{}"`
}

// CustomerCenterConfigurationSaveRes confirms the save.
type CustomerCenterConfigurationSaveRes struct {
	Saved bool `json:"saved" dc:"Settings were saved" eg:"true"`
}
