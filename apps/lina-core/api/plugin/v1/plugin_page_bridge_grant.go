package v1

import "github.com/gogf/gf/v2/frame/g"

// PageBridgeGrantReq asks the host to authorize one exact isolated-page API call.
type PageBridgeGrantReq struct {
	g.Meta         `path:"/plugins/page-bridge/grant" method:"post" tags:"Plugin Management" summary:"Issue isolated plugin page API grant" dc:"Validates the authenticated actor, active dynamic plugin release, governed frontend surface, exact capability, method, and route before issuing one short-lived opaque request grant."`
	PluginId       string `json:"pluginId" v:"required" dc:"Dynamic plugin owning the isolated page" eg:"phone-provider"`
	Host           string `json:"host" v:"required|in:admin,customer" dc:"Host surface identity: admin or customer" eg:"customer"`
	SurfaceKey     string `json:"surfaceKey" v:"required" dc:"Governed frontend surface key" eg:"phone-orders"`
	SurfaceVersion string `json:"surfaceVersion" v:"required" dc:"Governed frontend surface contract version" eg:"v1"`
	Capability     string `json:"capability" v:"required" dc:"Declared page bridge capability" eg:"phone-orders.query-own"`
	Method         string `json:"method" v:"required|in:GET,POST,PUT,PATCH,DELETE" dc:"Exact plugin API HTTP method: GET, POST, PUT, PATCH, or DELETE" eg:"GET"`
	Path           string `json:"path" v:"required" dc:"Plugin-relative API path without the host namespace" eg:"phone-orders"`
}

type PageBridgeGrantRes struct {
	Grant         string `json:"grant" dc:"Short-lived opaque signed page bridge grant" eg:"eyJ2IjoxLCJleHAiOjE3OTAwMDAwMDB9.signature"`
	ExpiresAt     int64  `json:"expiresAt" dc:"Grant expiry Unix timestamp in milliseconds" eg:"1790000000000"`
	OwnerPluginId string `json:"ownerPluginId" dc:"Server-resolved plugin namespace owning the exact API route; the isolated page cannot choose it" eg:"idc-core"`
}
