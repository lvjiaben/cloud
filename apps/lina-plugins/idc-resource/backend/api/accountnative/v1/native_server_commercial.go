package v1

import (
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
)

type NativeServerCommercialQuoteReq struct {
	g.Meta         `path:"/account/native-servers/{serverNo}/commercial-quotes" method:"post" tags:"IDC Account Servers" summary:"Quote my server commercial operation" dc:"Freeze the core owner's published price for this owned server; customer amounts, supplier identities and arbitrary IP or resize parameters are rejected."`
	ServerNo       string          `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	Action         string          `json:"action" v:"required|in:snapshot_create,backup_create,traffic_add,ip_change,resize" dc:"Commercial action: snapshot_create, backup_create, traffic_add, ip_change or resize" eg:"traffic_add"`
	Parameters     json.RawMessage `json:"parameters" dc:"Snapshot or backup inputs use diskObjectNo and name; other actions accept only an empty object" eg:"{}"`
	PackageNo      string          `json:"packageNo" v:"max-length:40" dc:"Published traffic package number; empty for other actions" eg:""`
	TargetQuoteNo  string          `json:"targetQuoteNo" v:"max-length:40" dc:"Owned structured target quote required for resize; empty for other actions" eg:""`
	IdempotencyKey string          `json:"idempotencyKey" v:"required|length:8,128|regex:^[A-Za-z0-9][A-Za-z0-9_.:-]*$" dc:"Request key retained for this exact commercial quote" eg:"commercial-quote-1"`
}

type NativeServerCommercialQuoteRes struct {
	QuoteNo               string `json:"quoteNo" dc:"Frozen commercial quote number" eg:"nqt_0123456789abcdef0123456789abcdef"`
	Action                string `json:"action" dc:"Frozen commercial action: snapshot_create, backup_create, traffic_add, ip_change or resize" eg:"traffic_add"`
	CurrencyCode          string `json:"currencyCode" dc:"Quote currency code" eg:"CNY"`
	TotalAmountMinor      string `json:"totalAmountMinor" dc:"Final inclusive amount as a decimal string in minor currency units" eg:"1000"`
	TaxAmountMinor        string `json:"taxAmountMinor" dc:"Tax portion already included in the final amount, as a decimal string in minor currency units" eg:"0"`
	ExpiresAt             int64  `json:"expiresAt" dc:"Quote expiry as Unix timestamp in milliseconds" eg:"1792382700000"`
	FreeCountPerServer    int    `json:"freeCountPerServer" dc:"Published free object allowance per server" eg:"1"`
	ExistingObjectCount   int    `json:"existingObjectCount" dc:"Current matching objects counted by the owner" eg:"1"`
	ChargedQuantity       int    `json:"chargedQuantity" dc:"Number of objects charged by this quote" eg:"1"`
	TrafficGB             int    `json:"trafficGb" dc:"Fixed traffic package capacity in GB; zero for non-traffic actions" eg:"100"`
	PeriodUnit            string `json:"periodUnit" dc:"Object billing period: day, month, year or empty for one-time actions" eg:"month"`
	PeriodCount           int    `json:"periodCount" dc:"Number of units in the object billing period; zero for one-time actions" eg:"1"`
	Direction             string `json:"direction" dc:"Resize direction: upgrade, downgrade or empty for other actions" eg:"upgrade"`
	EffectiveAt           *int64 `json:"effectiveAt,omitempty" dc:"Scheduled effect time as Unix timestamp in milliseconds; absent when not applicable" eg:"1792382700000"`
	CurrentRecurringMinor string `json:"currentRecurringMinor" dc:"Current recurring price as a decimal string in minor currency units for resize" eg:"1000"`
	TargetRecurringMinor  string `json:"targetRecurringMinor" dc:"Frozen target recurring price as a decimal string in minor currency units for resize" eg:"2000"`
	Reused                bool   `json:"reused" dc:"The same request returned its existing frozen quote" eg:"false"`
}

type NativeServerCommercialConfirmationReq struct {
	g.Meta         `path:"/account/native-servers/{serverNo}/commercial-confirmations" method:"post" tags:"IDC Account Servers" summary:"Prepare my server purchase confirmation" dc:"Resolve the frozen quote action and amount through core before preparing an exact current-session confirmation; caller action and amount are never trusted."`
	ServerNo       string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	QuoteNo        string `json:"quoteNo" v:"required|max-length:40" dc:"Owned frozen commercial quote number" eg:"nqt_0123456789abcdef0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,128|regex:^[A-Za-z0-9][A-Za-z0-9_.:-]*$" dc:"Request key retained for this exact checkout" eg:"commercial-checkout-1"`
}

type NativeServerCommercialConfirmationRes = NativeServerConfirmationRes

type NativeServerCommercialCheckoutReq struct {
	g.Meta         `path:"/account/native-servers/{serverNo}/commercial-purchases" method:"post" tags:"IDC Account Servers" summary:"Purchase my server commercial operation" dc:"Atomically bind the authoritative quote, required session confirmation, inventory and payable documents; provider writes remain behind financial settlement and the existing resource executor."`
	ServerNo       string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	QuoteNo        string `json:"quoteNo" v:"required|max-length:40" dc:"Owned frozen commercial quote number" eg:"nqt_0123456789abcdef0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,128|regex:^[A-Za-z0-9][A-Za-z0-9_.:-]*$" dc:"Request key retained for this exact checkout" eg:"commercial-checkout-1"`
	ProofToken     string `json:"proofToken" v:"max-length:4096" dc:"Current-session proof for the exact high-risk checkout; empty when not required" eg:""`
	Confirmed      bool   `json:"confirmed" v:"required" dc:"Customer explicitly accepted the operation impact and final amount" eg:"true"`
}

type NativeServerCommercialCheckoutRes struct {
	OperationNo      string `json:"operationNo" dc:"Core commercial operation number, not a supplier task identifier" eg:"BOP_0123456789abcdef0123456789abcdef"`
	QuoteNo          string `json:"quoteNo" dc:"Accepted frozen commercial quote number" eg:"nqt_0123456789abcdef0123456789abcdef"`
	OrderNo          string `json:"orderNo" dc:"Payable order number; empty for an authoritative free operation" eg:""`
	InvoiceNo        string `json:"invoiceNo" dc:"Invoice to pay; empty for an authoritative free operation" eg:""`
	Status           string `json:"status" dc:"Core operation state: awaiting_financial_settlement, financially_settled, ready, executing, succeeded, failed, canceled or manual_review" eg:"awaiting_financial_settlement"`
	CurrencyCode     string `json:"currencyCode" dc:"Purchase currency code" eg:"CNY"`
	TotalAmountMinor string `json:"totalAmountMinor" dc:"Final inclusive amount as a decimal string in minor currency units" eg:"1000"`
	LastErrorCode    string `json:"lastErrorCode" dc:"Safe commercial handling reason code; empty when unavailable" eg:""`
	Reused           bool   `json:"reused" dc:"The request returned the existing purchase without another charge" eg:"false"`
}

type NativeServerTrafficPackagesReq struct {
	g.Meta   `path:"/account/native-servers/{serverNo}/traffic-packages" method:"get" tags:"IDC Account Servers" summary:"List my server traffic packages" dc:"Read at most one hundred eligible published fixed traffic packages for this owned server; package validity follows the supplier reset policy rather than any quote or cache timestamp."`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
}

type NativeServerTrafficPackagesRes struct {
	Items []NativeServerTrafficPackage `json:"items" dc:"Eligible published fixed traffic packages" eg:"[]"`
}

type NativeServerTrafficPackage struct {
	PackageNo      string `json:"packageNo" dc:"Published traffic package number" eg:"POL_example"`
	NameZhCN       string `json:"nameZhCn" dc:"Chinese package display name" eg:"100GB Traffic"`
	NameEnUS       string `json:"nameEnUs" dc:"English package display name" eg:"100GB Traffic"`
	CurrencyCode   string `json:"currencyCode" dc:"Package currency code" eg:"CNY"`
	TrafficGB      int    `json:"trafficGb" dc:"Fixed package capacity in GB" eg:"100"`
	AmountMinor    string `json:"amountMinor" dc:"Final inclusive package price as a decimal string in minor currency units" eg:"1000"`
	RemainingStock *int64 `json:"remainingStock,omitempty" dc:"Remaining sellable packages; absent when stock is unlimited" eg:"10"`
	UnlimitedStock bool   `json:"unlimitedStock" dc:"The merchant does not limit package sale quantity" eg:"true"`
}
