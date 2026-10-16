// This file defines staff server edits that reuse customer pricing and immutable order workflows.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	account "lina-plugin-idc-core/backend/api/account/v1"
)

type CustomerServerMetadataGetReq struct {
	g.Meta    `path:"/admin/servers/{serviceNo}/metadata" method:"get" tags:"IDC Customer Products" summary:"Get server display information" dc:"Read one server alias, note and current version after staff and merchant-scope authorization." permission:"idc:admin:resource:server-accessory:view"`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Target server number" eg:"SVC_SERVER_0001"`
}
type CustomerServerMetadataGetRes struct {
	Metadata *account.ServerCustomerMetadata `json:"metadata" dc:"Safe server alias, note and version" eg:"{}"`
}
type CustomerServerMetadataUpdateReq struct {
	g.Meta          `path:"/admin/servers/{serviceNo}/metadata" method:"put" tags:"IDC Customer Products" summary:"Edit server display information" dc:"Update the owned server alias and note with an exact edit grant and version check; prices and paid orders remain immutable." permission:"idc:admin:resource:server:edit"`
	ServiceNo       string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Target server number" eg:"SVC_SERVER_0001"`
	Alias           string `json:"alias" v:"max-length:128" dc:"Short server display name" eg:"Production server"`
	Remark          string `json:"remark" v:"max-length:1000" dc:"Non-sensitive server note" eg:"Used for reports"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Current server version; stale edits are rejected" eg:"8"`
}
type CustomerServerMetadataUpdateRes = CustomerServerMetadataGetRes

type CustomerServerResizeOptionsReq struct {
	g.Meta    `path:"/admin/servers/{serviceNo}/resize-options" method:"get" tags:"IDC Customer Products" summary:"Get server configuration choices" dc:"Read current published configuration choices after exact staff edit permission and merchant-scope checks." permission:"idc:admin:resource:server:edit"`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Target server number" eg:"SVC_SERVER_0001"`
}
type CustomerServerResizeOptionsRes = account.ServerResizeOptionsRes

type CustomerServerResizeQuoteReq struct {
	g.Meta         `path:"/admin/servers/{serviceNo}/resize-quotes" method:"post" tags:"IDC Customer Products" summary:"Quote server configuration change" dc:"Reuse the original upgrade or downgrade workflow to freeze fees before customer payment; no historical order is rewritten." permission:"idc:admin:resource:server:edit"`
	ServiceNo      string                           `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Target server number" eg:"SVC_SERVER_0001"`
	Revision       string                           `json:"revision" v:"required|max-length:4096" dc:"Published configuration revision" eg:"rev_1"`
	CurrencyCode   string                           `json:"currencyCode" v:"required|length:3" dc:"Existing server currency" eg:"CNY"`
	Selections     []*account.ServerResizeSelection `json:"selections" v:"required" dc:"Chosen configuration options and quantities" eg:"[]"`
	IdempotencyKey string                           `json:"idempotencyKey" v:"required|min-length:8|max-length:256" dc:"Unique retry key" eg:"staff-resize-0001"`
}
type CustomerServerResizeQuoteRes = account.ServerResizeQuoteCreateRes

type CustomerServerResizeCheckoutReq struct {
	g.Meta         `path:"/admin/servers/{serviceNo}/resize-quotes/{quoteNo}/checkout" method:"post" tags:"IDC Customer Products" summary:"Confirm server configuration change" dc:"Confirm an unexpired quote for this server; upgrades create a pending customer bill and downgrades apply next renewal without refund." permission:"idc:admin:resource:server:edit"`
	ServiceNo      string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Target server number" eg:"SVC_SERVER_0001"`
	QuoteNo        string `json:"quoteNo" in:"path" v:"required|max-length:40" dc:"Displayed frozen configuration quote" eg:"rsq_0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:256" dc:"Unique retry key" eg:"staff-resize-checkout-0001"`
}
type CustomerServerResizeCheckoutRes = account.ServerResizeCheckoutCreateRes
