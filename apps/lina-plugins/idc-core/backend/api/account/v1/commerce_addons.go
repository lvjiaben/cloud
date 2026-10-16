// This file exposes only current-customer active add-on targets and public offers.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CommerceAddonTargetsReq struct {
	g.Meta `path:"/account/commerce/addon-targets" method:"get" tags:"IDC Customer Commerce" summary:"List my add-on targets" dc:"Return at most 100 active owned services and at most 100 allowed offers per service; expired and foreign services are excluded."`
}
type CommerceAddonOffer struct {
	PublicSlug string `json:"publicSlug" dc:"Public offer slug" eg:"backup"`
	NameZhCN   string `json:"nameZhCN" dc:"Simplified Chinese offer name" eg:"Backup"`
	NameEnUS   string `json:"nameEnUS" dc:"English offer name" eg:"Backup"`
}
type CommerceAddonTarget struct {
	EntitlementNo   string                `json:"entitlementNo" dc:"Owned active target service number" eg:"ent_service"`
	ProductNameZhCN string                `json:"productNameZhCN" dc:"Purchased main product name in Simplified Chinese" eg:"Cloud"`
	ProductNameEnUS string                `json:"productNameEnUS" dc:"Purchased main product name in English" eg:"Cloud"`
	Addons          []*CommerceAddonOffer `json:"addons" dc:"Allowed active offers for this service" eg:"[]"`
}
type CommerceAddonTargetsRes struct {
	Targets []*CommerceAddonTarget `json:"targets" dc:"Bounded active customer-owned add-on targets" eg:"[]"`
}
