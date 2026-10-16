// This file defines customer-owned upgrade discovery and safe projections.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ListReq struct {
	g.Meta `path:"/account/entitlement-upgrades" method:"get" tags:"IDC Customer Upgrades" summary:"List my service upgrades" dc:"Return bounded active local services and recent immutable upgrade intents owned by the current customer." pageBridgeCapability:"entitlement-upgrades.customer.v1" pageBridgeHost:"customer"`
}
type Target struct {
	EntitlementNo string `json:"entitlementNo" dc:"Owned service reference" eg:"ENT_1"`
	PublicSlug    string `json:"publicSlug" dc:"Published product slug" eg:"cloud-server"`
	NameZhCN      string `json:"nameZhCN" dc:"Chinese product name" eg:"Cloud server"`
	NameEnUS      string `json:"nameEnUS" dc:"English product name" eg:"Cloud server"`
	CurrencyCode  string `json:"currencyCode" dc:"Service currency" eg:"CNY"`
}
type Upgrade struct {
	UpgradeNo         string `json:"upgradeNo" dc:"Immutable upgrade reference" eg:"UPG_1"`
	EntitlementNo     string `json:"entitlementNo" dc:"Owned service reference" eg:"ENT_1"`
	CurrencyCode      string `json:"currencyCode" dc:"Exact invoice currency" eg:"CNY"`
	StatusCode        string `json:"statusCode" dc:"quoted, pending_payment, ready, executing, retry_wait, succeeded, manual_review or cancelled" eg:"quoted"`
	InvoiceNo         string `json:"invoiceNo" dc:"Payable invoice; empty before confirmation" eg:"UIN_1"`
	SourcePeriodMinor string `json:"sourcePeriodMinor" dc:"Tax-exclusive paid periodic basis in minor units" eg:"8000"`
	TargetPeriodMinor string `json:"targetPeriodMinor" dc:"Tax-exclusive upgraded full-period price in minor units" eg:"12000"`
	NetMinor          string `json:"netMinor" dc:"Prorated tax-exclusive charge in minor units" eg:"2000"`
	TaxMinor          string `json:"taxMinor" dc:"Additional tax in minor units" eg:"120"`
	TotalMinor        string `json:"totalMinor" dc:"Exact payable total in minor units" eg:"2120"`
	ExpiresAt         int64  `json:"expiresAt" dc:"Quote expiry as Unix timestamp in milliseconds" eg:"1792050000000"`
}
type ListRes struct {
	Targets []*Target  `json:"targets" dc:"At most 100 owned active services" eg:"[]"`
	Items   []*Upgrade `json:"items" dc:"At most 100 recent upgrade intents" eg:"[]"`
}
