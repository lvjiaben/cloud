// This file exposes only the active customer template public presentation settings.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-portal/backend/api/admin/v1"
	"lina-plugin-idc-portal/backend/cap/templatecap"
)

// CustomerCenterConfigurationGetReq reads brand and safe template settings without authentication.
type CustomerCenterConfigurationGetReq struct {
	g.Meta `path:"/public/customer-center/configuration" method:"get" tags:"Customer Center" summary:"Get active customer center template" dc:"Read only public brand, active template identity and public template configuration; no customer or secret values."`
}

// CustomerCenterConfigurationGetRes returns one active public template snapshot.
type CustomerCenterConfigurationGetRes struct {
	Entry          *templatecap.Entry      `json:"entry,omitempty" dc:"Published static application entry" eg:"{}"`
	Active         bool                    `json:"active" dc:"Whether the selected template is installed and enabled" eg:"true"`
	SelectedPlugin string                  `json:"selectedPlugin" dc:"Active customer template identity" eg:"customer-default"`
	Common         v1.CustomerCenterCommon `json:"common" dc:"Public customer center brand" eg:"{}"`
	Config         map[string]any          `json:"config" dc:"Public template-specific fields" eg:"{}"`
	Renderer       string                  `json:"renderer" dc:"Declared rendering contract" eg:"customer-standard-v1"`
	EntryURL       string                  `json:"entryUrl" dc:"Host-governed active template static entry" eg:"/x-assets/customer-default/v0.1.0/index.html"`
}
