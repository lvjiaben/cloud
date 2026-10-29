package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	publicv1 "lina-plugin-idc-core/backend/api/public/v1"
)

type CommerceAssistedQuoteItemInput struct {
	Values                []*publicv1.PurchaseConfigurationValue `json:"values" dc:"Bounded explicit typed value rows" eg:"[]"`
	Quantity              int                                    `json:"quantity" v:"required|min:1|max:1000" dc:"Number of product units"`
	PublicSlug            string                                 `json:"publicSlug" v:"required|length:1,128" dc:"Public product slug"`
	ConfigurationRevision string                                 `json:"configurationRevision" v:"required|length:43,43" dc:"Opaque published configuration revision"`
	BillingTermToken      string                                 `json:"billingTermToken" v:"required|length:1,128" dc:"Opaque selected billing term token"`
	PurchaseCycles        int                                    `json:"purchaseCycles" v:"required|min:1|max:120" dc:"Requested billing cycles"`
}

type CommerceAssistedQuoteItem struct {
	RequestItemNo   string `json:"requestItemNo" dc:"Stable assisted-request line number"`
	LineNumber      int    `json:"lineNumber" dc:"Stable display order"`
	ProductNameZhCN string `json:"productNameZhCN" dc:"Frozen Simplified Chinese product name"`
	ProductNameEnUS string `json:"productNameEnUS" dc:"Frozen English product name"`
	Quantity        int    `json:"quantity" dc:"Number of product units"`
	PurchaseCycles  int    `json:"purchaseCycles" dc:"Requested billing cycles"`
}

type CommerceAssistedQuote struct {
	RequestNo            string `json:"requestNo" dc:"Stable assisted quote request number" eg:"AQR_20260811_0001"`
	CustomerNo           string `json:"customerNo" dc:"Target customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	CustomerCode         string `json:"customerCode" dc:"Customer-facing short code" eg:"10005"`
	CustomerDisplayName  string `json:"customerDisplayName" dc:"Safe customer display name" eg:"Example customer"`
	CustomerPrimaryPhone string `json:"customerPrimaryPhone" dc:"Safe primary phone summary" eg:"+8613800138005"`
	CurrencyCode         string `json:"currencyCode" dc:"Exact configured currency" eg:"CNY"`
	StatusCode           string `json:"statusCode" dc:"ordered: directly created order; direct_ready: internal direct-checkout state; pending_approval, approved, rejected: read-only historical approval states" eg:"ordered"`
	ManualDiscountMinor  string `json:"manualDiscountMinor" dc:"Requested fixed discount in minor units" eg:"10000"`
	PaymentTermCode      string `json:"paymentTermCode" dc:"Prepayment deadline policy" eg:"extended_prepay"`
	PaymentDueSeconds    int    `json:"paymentDueSeconds" dc:"Frozen order payment window in seconds" eg:"172800"`

	Reason           string                       `json:"reason" dc:"Staff business reason" eg:"Customer retention quotation request"`
	InternalNote     string                       `json:"internalNote" dc:"Internal staff note" eg:"Customer requested assistance choosing a product"`
	RequestedBy      int                          `json:"requestedBy" dc:"Requesting staff user ID" eg:"1001"`
	DecisionCode     string                       `json:"decisionCode,omitempty" dc:"Read-only historical decision: approved or rejected; absent for directly created orders" eg:"approved"`
	DecisionBy       int                          `json:"decisionBy,omitempty" dc:"Read-only historical deciding staff user ID; absent for directly created orders" eg:"1002"`
	DecisionReason   string                       `json:"decisionReason,omitempty" dc:"Read-only historical decision reason; absent for directly created orders" eg:"Pricing and customer scope verified"`
	DecisionAt       int64                        `json:"decisionAt,omitempty" dc:"Read-only historical decision time as Unix timestamp in milliseconds" eg:"1786406460000"`
	QuoteNo          string                       `json:"quoteNo,omitempty" dc:"Immutable authoritative quote used to create the order" eg:"QUO_20260811_0001"`
	OrderNo          string                       `json:"orderNo,omitempty" dc:"Created order number" eg:"ORD_20260811_0001"`
	OrderStatusCode  string                       `json:"orderStatusCode,omitempty" dc:"Current order status" eg:"pending_payment"`
	SubtotalMinor    string                       `json:"subtotalMinor" dc:"Fresh authoritative quote subtotal in minor units" eg:"200000"`
	DiscountMinor    string                       `json:"discountMinor" dc:"Frozen order discount in currency minor units" eg:"10000"`
	TotalAmountMinor string                       `json:"totalAmountMinor" dc:"Fresh authoritative payable amount in minor units" eg:"190000"`
	ExpiresAt        int64                        `json:"expiresAt" dc:"Payment deadline as Unix timestamp in milliseconds" eg:"1786492800000"`
	CreatedAt        int64                        `json:"createdAt" dc:"Assistance creation time as Unix timestamp in milliseconds" eg:"1786406400000"`
	Items            []*CommerceAssistedQuoteItem `json:"items" dc:"Bounded immutable request lines" eg:"[]"`
	CanApprove       bool                         `json:"canApprove" dc:"Always false; historical approval actions are disabled" eg:"false"`
	CanReject        bool                         `json:"canReject" dc:"Always false; historical rejection actions are disabled" eg:"false"`
	CanCheckout      bool                         `json:"canCheckout" dc:"Always false; assisted orders are created directly, historical checkout actions are disabled" eg:"false"`
	Reused           bool                         `json:"reused" dc:"Whether an exact idempotent replay returned the committed result" eg:"false"`
}

type CommerceAssistedQuoteCreateReq struct {
	g.Meta              `path:"/admin/commerce/assisted-quotes" method:"post" tags:"IDC Commerce Management" summary:"Create a customer assisted order" dc:"Create the customer order directly using authoritative prices and normal checkout gates; never debit customer balance." permission:"idc:admin:commerce:assisted-quote:create"`
	CustomerNo          string `json:"customerNo" v:"required|max-length:40" dc:"Target customer number"`
	CurrencyCode        string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Explicit configured currency"`
	ManualDiscountMinor string `json:"manualDiscountMinor" v:"required|regex:^([0-9]|[1-9][0-9]{0,15})$" dc:"Requested fixed discount over current authoritative price"`
	PaymentDueAt        int64  `json:"paymentDueAt" v:"required|min:1" dc:"Payment deadline as Unix timestamp in milliseconds, five minutes to seven days in the future" eg:"1791626400000"`

	Reason         string                            `json:"reason" v:"max-length:1000" dc:"Optional staff note for the assisted order"`
	InternalNote   string                            `json:"internalNote" v:"max-length:2000" dc:"Optional internal note"`
	IdempotencyKey string                            `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random action key stable across retries"`
	Items          []*CommerceAssistedQuoteItemInput `json:"items" v:"required" dc:"One to eight product purchase lines; collection bounds are enforced by the commerce service"`
}

type CommerceAssistedQuoteCreateRes struct {
	Request *CommerceAssistedQuote `json:"request" dc:"Created customer order and immutable assistance record"`
}

type CommerceAssistedQuoteListReq struct {
	Page       int `json:"page" v:"min:0" dc:"Optional one-based page number; cannot be combined with cursor" eg:"1"`
	g.Meta     `path:"/admin/commerce/assisted-quotes" method:"get" tags:"IDC Commerce Management" summary:"List assisted order records" dc:"Return one bounded page of direct assistance and read-only historical approval records without exposing evidence digests." permission:"idc:admin:commerce:assisted-quote:list"`
	CustomerNo string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number"`
	StatusCode string `json:"statusCode" v:"in:,pending_approval,approved,rejected,ordered" dc:"Optional status: ordered for created orders; pending_approval, approved, rejected for read-only history; omitted returns all statuses"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page"`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum requests returned"`
}

type CommerceAssistedQuoteListRes struct {
	Total      int                      `json:"total" dc:"Total records matching the current filters and authorized data scope" eg:"0"`
	Requests   []*CommerceAssistedQuote `json:"requests" dc:"Bounded assisted quote summaries"`
	NextCursor string                   `json:"nextCursor" dc:"Encrypted next-page cursor, or empty"`
}

type CommerceAssistedQuoteDetailReq struct {
	g.Meta    `path:"/admin/commerce/assisted-quotes/{requestNo}" method:"get" tags:"IDC Commerce Management" summary:"Get assisted order record" dc:"Read the assistance record, authoritative quote and order; historical decisions remain read-only and evidence digests are excluded." permission:"idc:admin:commerce:assisted-quote:query"`
	RequestNo string `json:"requestNo" in:"path" v:"required|max-length:40" dc:"Stable assisted quote request number"`
}

type CommerceAssistedQuoteDetailRes struct {
	Request *CommerceAssistedQuote `json:"request" dc:"Assisted order record with read-only historical decisions when present"`
}

// CommerceAssistedOrderOptionsReq reads configured currency choices for one staff action.
type CommerceAssistedOrderOptionsReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/assisted-order-options" method:"get" tags:"IDC Commerce Management" summary:"Get assisted order options" dc:"Read the configured merchant currency for an authorized customer; product pricing remains independently validated." permission:"idc:admin:commerce:assisted-quote:create"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Target customer number" eg:"CUS_example"`
}
type CommerceAssistedOrderOptionsRes struct {
	Currencies []string `json:"currencies" dc:"Configured merchant currencies; product prices are never inferred" eg:"[&quot;CNY&quot;]"`
}
