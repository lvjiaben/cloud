// Package v1 contains channel request and response projections.
package v1

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"

	"github.com/gogf/gf/v2/frame/g"
)

type CatalogReq struct {
	NamePrefix string `json:"namePrefix" v:"max-length:160" dc:"Product name prefix; empty selects all available products" eg:"Cloud"`
	Currency   string `json:"currency" v:"max-length:3" dc:"Optional currency filter" eg:"CNY"`
	Cursor     string `json:"cursor" dc:"Opaque next-page cursor from the preceding response" eg:""`
	Limit      int    `json:"limit" d:"24" v:"min:1|max:24" dc:"Maximum products in this page" eg:"24"`
	g.Meta     `path:"/channel/catalog" method:"get" tags:"Downstream Channel" summary:"List sanitized channel catalog"`
}
type CatalogItem struct {
	ProductVersionId int64  `json:"productVersionId"`
	Currency         string `json:"currency"`
	Price            string `json:"price"`
	Stock            int    `json:"stock"`
}
type CatalogRes struct {
	channelcommercecap.CatalogPage
}
type QuoteReq struct {
	g.Meta    `path:"/channel/quotes" method:"post" tags:"Open API" summary:"Create a frozen customer quote" dc:"Freeze the authenticated customer's product configuration and price for a short time. Reuse the request identifier only with identical input."`
	RequestId string                      `json:"requestId" v:"required|length:8,120" dc:"Unique idempotency identifier for the quote" eg:"quote-20261010-0001"`
	Purchase  channelcommercecap.Purchase `json:"purchase" dc:"Selected product, billing term, currency, quantity and configuration" eg:"{}"`
}

type QuoteRes struct {
	QuoteNo          string `json:"quoteNo" dc:"Frozen quote identifier required when ordering" eg:"CHQ_example"`
	ExpiresAt        int64  `json:"expiresAt" dc:"Quote expiry as a Unix timestamp in milliseconds" eg:"1791600000000"`
	ProductVersionId int64  `json:"productVersionId"`
	Currency         string `json:"currency"`
	Price            string `json:"price"`
	TotalAmountMinor int64  `json:"totalAmountMinor"`
	Stock            int    `json:"stock"`
	Status           string `json:"status"`
}
type DelegatedCustomerInput struct {
	CustomerType     string `json:"customerType" v:"in:individual,enterprise"`
	DisplayName      string `json:"displayName"`
	CountryCode      string `json:"countryCode"`
	PreferredLocale  string `json:"preferredLocale"`
	Email            string `json:"email"`
	PhoneCountryCode string `json:"phoneCountryCode"`
	Phone            string `json:"phone"`
}
type CreateOrderReq struct {
	g.Meta    `path:"/channel/orders" method:"post" tags:"Open API" summary:"Purchase a frozen quote using customer balance" dc:"Create the customer order and pay the exact same-currency amount atomically. Insufficient balance leaves no order or stock reservation."`
	RequestId string `json:"requestId" v:"required|length:8,120" dc:"Unique idempotency identifier; retries must keep identical input" eg:"order-20261010-0001"`
	QuoteNo   string `json:"quoteNo" v:"required|max-length:40" dc:"Unexpired frozen quote returned by POST channel quotes" eg:"CHQ_example"`
}

type CreateOrderRes struct {
	Id       int64  `json:"id"`
	Status   string `json:"status"`
	Replayed bool   `json:"replayed"`
}
type GetOrderReq struct {
	g.Meta `path:"/channel/orders/{id}" method:"get" tags:"Downstream Channel" summary:"Get channel order"`
	Id     int64 `json:"id" v:"required"`
}
type GetOrderRes struct {
	Id                     int64  `json:"id"`
	ChannelId              string `json:"channelId"`
	OwnershipMode          string `json:"ownershipMode"`
	Currency               string `json:"currency"`
	Status                 string `json:"status"`
	FinanciallySettled     bool   `json:"financiallySettled"`
	IdentityReady          bool   `json:"identityReady"`
	CoreOrderNo            string `json:"coreOrderNo"`
	PayerCustomerNo        string `json:"payerCustomerNo"`
	ServiceOwnerCustomerNo string `json:"serviceOwnerCustomerNo"`
	TotalAmountMinor       int64  `json:"totalAmountMinor"`
}
type LifecycleReq struct {
	g.Meta     `path:"/channel/services/{serviceNo}/operations/{operation}" method:"post" tags:"Downstream Channel" summary:"Submit a versioned server lifecycle operation" dc:"Submit renew, reconfigure, reinstall, console, change IP or terminate using the shared lifecycle envelope. Channel identity is derived only from authenticated AppID/AppSecret."`
	ServiceNo  string          `json:"serviceNo" v:"required" dc:"Opaque service identifier scoped to the channel" eg:"svc_10001"`
	RequestId  string          `json:"requestId" v:"required|length:8,120" dc:"Idempotency key for this lifecycle request" eg:"lifecycle-20260830-001"`
	Operation  string          `json:"operation" v:"required|in:renew,reconfigure,reinstall,console,change_ip,terminate" dc:"Lifecycle operation: renew, reconfigure, reinstall, console, change_ip or terminate" eg:"renew"`
	Parameters json.RawMessage `json:"parameters" dc:"Operation-specific JSON parameters; validated by the resource owner when wired" eg:"{}"`
	Confirmed  bool            `json:"confirmed" dc:"Explicit confirmation for high-risk operations" eg:"true"`
}
type LifecycleRes struct {
	OperationNo     string `json:"operationNo" dc:"Opaque operation number when accepted" eg:"SOP_10001"`
	QuoteNo         string `json:"quoteNo" dc:"Opaque authoritative quote number for a priced operation"`
	Status          string `json:"status" dc:"Normalized lifecycle status" eg:"pending"`
	CurrencyCode    string `json:"currencyCode" dc:"Explicit quote currency"`
	Direction       string `json:"direction" dc:"Resize direction: upgrade, downgrade or equal"`
	AmountMinor     int64  `json:"amountMinor" dc:"Authoritative amount in minor currency units"`
	EffectiveAt     *int64 `json:"effectiveAt" dc:"Authoritative effective time for a resize as a Unix timestamp in milliseconds" eg:"1788163200000"`
	ExpiresAt       *int64 `json:"expiresAt" dc:"Quote expiry time as a Unix timestamp in milliseconds" eg:"1788164100000"`
	PaymentRequired bool   `json:"paymentRequired" dc:"Whether the operation is awaiting same-currency balance settlement"`
	OutputAvailable bool   `json:"outputAvailable" dc:"Whether a completed console or reinstall operation has a confidential one-time output" eg:"false"`
	Replayed        bool   `json:"replayed" dc:"Whether the idempotency key reused an earlier result" eg:"false"`
}

type LifecycleOutputReq struct {
	g.Meta      `path:"/channel/services/{serviceNo}/operations/{operationNo}/confidential-output" method:"post" tags:"Downstream Channel" summary:"Reveal one completed lifecycle output once" dc:"Consume one console ticket or reinstall credential through a no-store response after explicit confirmation."`
	ServiceNo   string `json:"serviceNo" v:"required|max-length:40"`
	OperationNo string `json:"operationNo" v:"required|max-length:40"`
	Confirmed   bool   `json:"confirmed" v:"required"`
}

type LifecycleOutputRes struct {
	Payload json.RawMessage `json:"payload" dc:"One-time credential or short-lived console ticket written only to a no-store response"`
}
type CancelOrderReq struct {
	g.Meta    `path:"/channel/orders/{id}/cancel" method:"post" tags:"Downstream Channel" summary:"Cancel an order before fulfillment starts"`
	Id        int64  `json:"id" v:"required"`
	RequestId string `json:"requestId" v:"required|length:8,120"`
}
type CancelOrderRes struct {
	Id       int64  `json:"id"`
	Status   string `json:"status"`
	Replayed bool   `json:"replayed"`
}
type FinanceReq struct {
	g.Meta `path:"/channel/finance" method:"get" tags:"Downstream Channel" summary:"Get per-currency channel settlement overview"`
}
type FinanceCurrency struct {
	CurrencyCode          string `json:"currencyCode"`
	BalanceMinor          int64  `json:"balanceMinor"`
	HeldBalanceMinor      int64  `json:"heldBalanceMinor"`
	AvailableBalanceMinor int64  `json:"availableBalanceMinor"`
	CreditLimitMinor      int64  `json:"creditLimitMinor"`
	CreditAvailableMinor  int64  `json:"creditAvailableMinor"`
	ReceivableMinor       int64  `json:"receivableMinor"`
	PaidMinor             int64  `json:"paidMinor"`
	RefundedMinor         int64  `json:"refundedMinor"`
	ServiceCostMinor      int64  `json:"serviceCostMinor"`
}
type FinanceRes struct {
	Currencies       []*FinanceCurrency `json:"currencies"`
	Invoices         []*FinanceInvoice  `json:"invoices"`
	CreditStatements []*CreditStatement `json:"creditStatements"`
	CreditRepayments []*CreditRepayment `json:"creditRepayments"`
}
type FinanceInvoice struct {
	InvoiceNo           string `json:"invoiceNo"`
	OrderNo             string `json:"orderNo"`
	CurrencyCode        string `json:"currencyCode"`
	Status              string `json:"status"`
	TotalAmountMinor    int64  `json:"totalAmountMinor"`
	PaidAmountMinor     int64  `json:"paidAmountMinor"`
	CreditAmountMinor   int64  `json:"creditAmountMinor"`
	RefundedAmountMinor int64  `json:"refundedAmountMinor"`
	CreatedAt           int64  `json:"createdAt" dc:"Invoice creation time as a Unix timestamp in milliseconds" eg:"1788163200000"`
}
type CreditStatement struct {
	StatementNo      string `json:"statementNo"`
	CurrencyCode     string `json:"currencyCode"`
	Status           string `json:"status"`
	TotalMinor       int64  `json:"totalMinor"`
	PaidMinor        int64  `json:"paidMinor"`
	OutstandingMinor int64  `json:"outstandingMinor"`
	IssuedAt         int64  `json:"issuedAt" dc:"Statement issue time as a Unix timestamp in milliseconds" eg:"1788163200000"`
	PaymentDueAt     int64  `json:"paymentDueAt" dc:"Payment deadline as a Unix timestamp in milliseconds" eg:"1790755200000"`
}
type CreditRepayment struct {
	RepaymentNo    string `json:"repaymentNo"`
	StatementNo    string `json:"statementNo"`
	CurrencyCode   string `json:"currencyCode"`
	Status         string `json:"status"`
	AmountMinor    int64  `json:"amountMinor"`
	AllocatedMinor int64  `json:"allocatedMinor"`
	CreatedAt      int64  `json:"createdAt" dc:"Repayment creation time as a Unix timestamp in milliseconds" eg:"1788163200000"`
}
type ListServicesReq struct {
	OrderNo        string `json:"orderNo" v:"max-length:40" dc:"Filter by the core order number; empty includes all customer application orders" eg:"ORD_10001"`
	AfterServiceNo string `json:"afterServiceNo" v:"max-length:40" dc:"Service number from the previous page; empty starts at the first page" eg:"ENT_10001"`
	Limit          int    `json:"limit" d:"100" v:"min:1|max:100" dc:"Maximum services returned; continue after the last service number" eg:"100"`
	g.Meta         `path:"/channel/services" method:"get" tags:"Downstream Channel" summary:"List bounded historical channel services"`
}
type GetServiceReq struct {
	g.Meta    `path:"/channel/services/{serviceNo}" method:"get" tags:"Downstream Channel" summary:"Get one channel service"`
	ServiceNo string `json:"serviceNo" v:"required|length:1,40"`
}
type ServiceItem struct {
	ServiceNo              string `json:"serviceNo"`
	ServiceOwnerCustomerNo string `json:"serviceOwnerCustomerNo"`
	OrderNo                string `json:"orderNo"`
	ProductNo              string `json:"productNo"`
	VersionNo              string `json:"versionNo"`
	CurrencyCode           string `json:"currencyCode"`
	Status                 string `json:"status"`
	RenewalMode            string `json:"renewalMode"`
	StartedAt              int64  `json:"startedAt" dc:"Service start time as a Unix timestamp in milliseconds" eg:"1788163200000"`
	ExpiresAt              *int64 `json:"expiresAt" dc:"Optional service expiry as a Unix timestamp in milliseconds" eg:"1819699200000"`
	UpdatedAt              int64  `json:"updatedAt" dc:"Last service update time as a Unix timestamp in milliseconds" eg:"1788166800000"`
}
type ListServicesRes struct {
	List []*ServiceItem `json:"list"`
}
type GetServiceRes struct {
	Service *ServiceItem `json:"service"`
}
type CapabilitiesReq struct {
	g.Meta `path:"/channel/capabilities" method:"get" tags:"Downstream Channel" summary:"List verified compatibility capabilities"`
}
type CapabilitiesRes struct {
	ObservedAt      int64             `json:"observedAt" dc:"Authenticated server observation time as Unix milliseconds" eg:"1791676800000"`
	GenericAPI      bool              `json:"genericApi"`
	WebhookDelivery bool              `json:"webhookDelivery"`
	Adapters        map[string]string `json:"adapters"`
	Operations      map[string]string `json:"operations"`
}

// GetWebhookReq addresses one immutable event in the channel scope derived
// from the bearer token. The caller cannot supply a different channel id.
type GetWebhookReq struct {
	g.Meta  `path:"/channel/webhooks/{eventId}" method:"get" tags:"Downstream Channel" summary:"Replay one versioned channel webhook event"`
	EventId string `json:"eventId" v:"required|length:68,120"`
}

type GetWebhookRes struct {
	EventId   string          `json:"eventId"`
	Version   string          `json:"version"`
	EventType string          `json:"eventType"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
	Status    string          `json:"status"`
	Attempts  int             `json:"attempts"`
}
