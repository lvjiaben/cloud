package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type CatalogNumberedReq struct {
	g.Meta     `path:"/channel/catalog/products" method:"get" tags:"Open API" summary:"List a numbered product catalog page" dc:"Bounded product page with the exact published-product count for import pickers; requires catalog.read."`
	NamePrefix string `json:"namePrefix" v:"max-length:160" dc:"Product name prefix" eg:"Cloud"`
	Currency   string `json:"currency" v:"max-length:3" dc:"Optional supported ISO currency filter" eg:"CNY"`
	Page       int    `json:"page" d:"1" v:"min:1|max:1000000" dc:"One-based page number" eg:"1"`
	PageSize   int    `json:"pageSize" d:"24" v:"min:1|max:100" dc:"Maximum products per page" eg:"24"`
}
type CatalogNumberedRes struct {
	channelcommercecap.CatalogNumberedPage
}

type ProductSnapshotReq struct {
	g.Meta     `path:"/channel/catalog/products/{publicSlug}/snapshot" method:"get" tags:"Open API" summary:"Read a same-system product import snapshot" dc:"Return safe configuration tokens and currency precision. The initial view must be remotely evaluated after changes; no price is invented for an unselected configuration."`
	PublicSlug string `json:"publicSlug" v:"required|max-length:160" dc:"Published product slug" eg:"cloud-server"`
}
type ProductSnapshotRes struct {
	channelcommercecap.ProductSnapshot
}

type GetOrderByRequestReq struct {
	g.Meta    `path:"/channel/orders/by-request/{requestId}" method:"get" tags:"Open API" summary:"Find an order by its original request identifier" dc:"Read only the authenticated application's completed order submission. Wait for a concurrent submission transaction before reporting absence; never resubmit or deduct balance."`
	RequestId string `json:"requestId" v:"required|length:8,120" dc:"Exact identifier sent to POST channel orders" eg:"peer-order-example"`
}
type GetOrderByRequestRes struct {
	Found bool         `json:"found" dc:"Whether this application has a committed order for the request" eg:"true"`
	Order *GetOrderRes `json:"order" dc:"Current customer-owned order, or null when absent" eg:"{}"`
}

// ImportSnapshotReq exports complete portable sale facts to an authenticated customer.
type ImportSnapshotReq struct {
	g.Meta     `path:"/channel/catalog/products/{publicSlug}/import-snapshot" method:"get" tags:"Open API" summary:"Read complete portable product facts" dc:"Read the complete safe schema and explicit customer price rules for importing a draft. Requires catalog.read; local publication and pricing remain explicit merchant actions."`
	PublicSlug string `json:"publicSlug" v:"required|max-length:160" dc:"Published product slug" eg:"cloud-server"`
}
type ImportSnapshotRes struct {
	channelcommercecap.ImportSnapshot
}

type ImportCandidatesReq struct {
	g.Meta                `path:"/channel/catalog/products/{publicSlug}/import-candidates" method:"get" tags:"Open API" summary:"Read dynamic import candidates" dc:"Refresh candidate options for the exact imported configuration without exposing provider credentials."`
	PublicSlug            string `json:"publicSlug" v:"required|max-length:160" dc:"Product identifier"`
	ConfigurationRevision string `json:"configurationRevision" v:"required|max-length:4096" dc:"Exact imported configuration revision"`
	Values                string `json:"values" v:"max-length:32768" dc:"URL-encoded JSON array of public typed selections; at most 128 items and 32768 UTF-8 bytes; omitted means no selections" eg:"[]"`
}
type ImportCandidatesRes struct {
	channelcommercecap.ImportCandidates
}

// OrderDeliveriesReq reads all customer-owned delivery facts for one API order.
type OrderDeliveriesReq struct {
	g.Meta `path:"/channel/orders/{id}/deliveries" method:"get" tags:"Open API" summary:"Read order delivery results" dc:"Read one-time delivery results and optional managed services without assuming every order is a server. Requires order.read."`
	Id     int64 `json:"id" v:"required|min:1" dc:"API order identifier returned by order creation" eg:"1"`
}
type OrderDeliveriesRes struct {
	ObservedAt int64           `json:"observedAt" dc:"Authenticated server observation time as Unix milliseconds" eg:"1791676800000"`
	OrderNo    string          `json:"orderNo" dc:"Core order identifier" eg:"ORD_10001"`
	Items      []OrderDelivery `json:"items" dc:"Complete bounded delivery results for this order" eg:"[]"`
}

type OrderDelivery struct {
	DeliveryNo  string            `json:"deliveryNo" dc:"Delivery identifier" eg:"DLV_10001"`
	Status      string            `json:"status" dc:"Delivery status: pending, running, completed, failed or cancelled" eg:"completed"`
	Quantity    int               `json:"quantity" dc:"Delivered quantity" eg:"1"`
	Ongoing     bool              `json:"ongoing" dc:"Whether the delivery requires ongoing service management" eg:"true"`
	StartedAt   *int64            `json:"startedAt" dc:"Delivery start as Unix timestamp in milliseconds; null when not started" eg:"1791676800000"`
	CompletedAt *int64            `json:"completedAt" dc:"Delivery completion as Unix timestamp in milliseconds; null when incomplete" eg:"1791676800000"`
	Services    []DeliveryService `json:"services" dc:"All managed services linked to the delivery" eg:"[]"`
}

type DeliveryService struct {
	ServiceNo    string `json:"serviceNo" dc:"Managed service identifier" eg:"ENT_10001"`
	Status       string `json:"status" dc:"Service status: pending, active, suspended, expired or terminated" eg:"active"`
	CurrencyCode string `json:"currencyCode" dc:"Frozen ISO service currency" eg:"CNY"`
	PeriodUnit   string `json:"periodUnit" dc:"Frozen billing unit: hour, day, week, month or year" eg:"month"`
	PeriodCount  int    `json:"periodCount" dc:"Frozen billing interval count" eg:"1"`
	RenewalMode  string `json:"renewalMode" dc:"Renewal mode: unavailable, local_entitlement or provider_confirmed" eg:"provider_confirmed"`
	StartedAt    *int64 `json:"startedAt" dc:"Service start as Unix timestamp in milliseconds; null when not started" eg:"1791676800000"`
	ExpiresAt    *int64 `json:"expiresAt" dc:"Service expiry as Unix timestamp in milliseconds; null when unlimited" eg:"1794355200000"`
}
