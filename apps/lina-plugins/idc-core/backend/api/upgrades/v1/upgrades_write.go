// This file defines upgrade quote, invoice confirmation and exact balance payment requests.
package v1

import "github.com/gogf/gf/v2/frame/g"

type QuoteReq struct {
	g.Meta         `path:"/account/entitlements/{entitlementNo}/upgrades" method:"post" tags:"IDC Customer Upgrades" summary:"Quote my configuration upgrade" dc:"Bind one owned compiler-frozen sales quote to an active paid service; derive the remaining-period difference without installation or a debit." pageBridgeCapability:"entitlement-upgrades.customer.v1" pageBridgeHost:"customer"`
	EntitlementNo  string `json:"entitlementNo" in:"path" v:"required|max-length:40" dc:"Owned service" eg:"ENT_1"`
	QuoteNo        string `json:"quoteNo" v:"required|max-length:40" dc:"Owned single-item quote with frozen target configuration" eg:"QUO_1"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,256" dc:"Stable key for exact retry" eg:"upgrade-12345678"`
}
type QuoteRes struct {
	Upgrade *Upgrade `json:"upgrade" dc:"Frozen difference quote" eg:"{}"`
}
type ConfirmReq struct {
	g.Meta    `path:"/account/entitlement-upgrades/{upgradeNo}/confirmations" method:"post" tags:"IDC Customer Upgrades" summary:"Confirm my upgrade" dc:"Create one owned invoice after rechecking service revision, expiry and quote; zero-amount upgrades queue without a balance debit." pageBridgeCapability:"entitlement-upgrades.customer.v1" pageBridgeHost:"customer"`
	UpgradeNo string `json:"upgradeNo" in:"path" v:"required|max-length:40" dc:"Owned upgrade reference" eg:"UPG_1"`
}
type ConfirmRes struct {
	Upgrade *Upgrade `json:"upgrade" dc:"Invoice and execution state" eg:"{}"`
}
type PayReq struct {
	g.Meta    `path:"/account/entitlement-upgrades/{upgradeNo}/balance-payments" method:"post" tags:"IDC Customer Upgrades" summary:"Pay my upgrade" dc:"Atomically debit only the upgrade invoice owner's same-currency balance and queue the already frozen Provider operation; retries reuse the invoice." pageBridgeCapability:"entitlement-upgrades.customer.v1" pageBridgeHost:"customer"`
	UpgradeNo string `json:"upgradeNo" in:"path" v:"required|max-length:40" dc:"Owned upgrade reference" eg:"UPG_1"`
}
type PayRes struct {
	Upgrade *Upgrade `json:"upgrade" dc:"Paid upgrade state" eg:"{}"`
}
