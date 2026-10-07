package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type ServerAccessoryQuote struct {
	TrafficGB           int    `json:"trafficGb" dc:"Frozen traffic package capacity in GB; zero for other accessories"`
	PackageNameZhCN     string `json:"packageNameZhCN" dc:"Frozen Chinese traffic package name"`
	PackageNameEnUS     string `json:"packageNameEnUS" dc:"Frozen English traffic package name"`
	QuoteNo             string `json:"quoteNo" dc:"Stable accessory quote number"`
	Action              string `json:"action" dc:"Quoted accessory action code"`
	Kind                string `json:"kind" dc:"Quoted accessory kind"`
	CurrencyCode        string `json:"currencyCode" dc:"Explicit quote currency"`
	RequestedQuantity   int    `json:"requestedQuantity" dc:"Requested object or fixed package count"`
	ExistingObjectCount int    `json:"existingObjectCount" dc:"Current objects of the same kind on this server"`
	ReservedCreateCount int    `json:"reservedCreateCount" dc:"Unfinished creates already reserving this server quota"`
	FreeCountPerServer  int    `json:"freeCountPerServer" dc:"Published free object count for each server"`
	ChargedQuantity     int    `json:"chargedQuantity" dc:"Quantity charged by this quote"`
	UnitAmountMinor     int64  `json:"unitAmountMinor" dc:"Published unit amount in minor currency units"`
	SubtotalMinor       int64  `json:"subtotalMinor" dc:"Frozen amount before the included tax split"`
	NetAmountMinor      int64  `json:"netAmountMinor" dc:"Frozen net amount inside the final customer price"`
	TaxAmountMinor      int64  `json:"taxAmountMinor" dc:"Frozen tax amount included in the final customer price"`
	TotalAmountMinor    int64  `json:"totalAmountMinor" dc:"Frozen quote total in minor currency units"`
	PriceIncludesTax    bool   `json:"priceIncludesTax" dc:"Whether the displayed final customer price already includes tax"`
	PeriodUnitCode      string `json:"periodUnitCode" dc:"Day, month, year, or once"`
	PeriodCount         int    `json:"periodCount" dc:"Number of period units"`
	PolicyVersion       int64  `json:"policyVersion" dc:"Published price policy version"`
	IssuedAt            int64  `json:"issuedAt" dc:"Quote issue time as Unix milliseconds"`
	ExpiresAt           int64  `json:"expiresAt" dc:"Quote expiry time as Unix milliseconds"`
}

type ServerAccessoryQuoteCreateReq struct {
	PackageNo      string `json:"packageNo" v:"max-length:40" dc:"Selected fixed traffic package; required only for traffic purchases"`
	g.Meta         `path:"/account/servers/{serviceNo}/accessory-quotes" method:"post" tags:"IDC Customer Servers" summary:"Preview my server accessory charge" dc:"Create an immutable short-lived quote from the current server-owned accessory directory, reserved creates, and merchant-published price."`
	ServiceNo      string          `json:"serviceNo" in:"path" v:"required|length:1,40" dc:"Stable server service number owned by the current customer"`
	Action         string          `json:"action" v:"required|in:snapshot_create,backup_create,traffic_add" dc:"Accessory creation or traffic-package action to quote"`
	Parameters     json.RawMessage `json:"parameters" v:"required" dc:"Action parameters validated by the frozen provider capability"`
	IdempotencyKey string          `json:"idempotencyKey" v:"required|length:8,256" dc:"Client-generated key used only to safely replay the same quote request"`
}

type ServerAccessoryQuoteCreateRes struct {
	Quote *ServerAccessoryQuote `json:"quote" dc:"Created or safely replayed immutable accessory quote"`
}

type ServerAccessoryCheckout struct {
	OperationNo     string `json:"operationNo" dc:"Stable server accessory operation number"`
	OrderNo         string `json:"orderNo" dc:"Order number when payment is required"`
	InvoiceNo       string `json:"invoiceNo" dc:"Invoice number when payment is required"`
	ServiceNo       string `json:"serviceNo" dc:"Owned server service number"`
	Status          string `json:"status" dc:"Current accessory operation status"`
	AmountMinor     int64  `json:"amountMinor" dc:"Final tax-inclusive amount in minor currency units"`
	CurrencyCode    string `json:"currencyCode" dc:"Exact uppercase three-letter currency code"`
	PaymentDueAt    int64  `json:"paymentDueAt" dc:"Payment deadline as Unix milliseconds"`
	PaymentRequired bool   `json:"paymentRequired" dc:"Whether payment must settle before execution"`
	Reused          bool   `json:"reused" dc:"Whether the same checkout request was safely replayed"`
}

type ServerAccessoryCheckoutCreateReq struct {
	g.Meta         `path:"/account/server-accessory-quotes/{quoteNo}/checkout" method:"post" tags:"IDC Customer Servers" summary:"Checkout my server accessory quote" dc:"Create a free execution or an ordinary payable order and invoice from my current immutable server accessory quote."`
	QuoteNo        string `json:"quoteNo" in:"path" v:"required|length:1,40" dc:"Current immutable accessory quote number owned by this customer"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,256" dc:"Client-generated key used only to safely replay the same checkout"`
}

type ServerAccessoryCheckoutCreateRes struct {
	Checkout *ServerAccessoryCheckout `json:"checkout" dc:"Free execution or payable order created from the quote"`
}
