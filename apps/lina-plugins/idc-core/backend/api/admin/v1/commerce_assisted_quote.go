package v1

import "github.com/gogf/gf/v2/frame/g"

type CommerceAssistedQuoteItemInput struct {
	Quantity              int    `json:"quantity" v:"required|min:1|max:1000" dc:"Number of product units"`
	PublicSlug            string `json:"publicSlug" v:"required|length:1,128" dc:"Public product slug"`
	ConfigurationRevision string `json:"configurationRevision" v:"required|length:43,43" dc:"Opaque published configuration revision"`
	BillingTermToken      string `json:"billingTermToken" v:"required|length:1,128" dc:"Opaque selected billing term token"`
	PurchaseCycles        int    `json:"purchaseCycles" v:"required|min:1|max:120" dc:"Requested billing cycles"`
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
	RequestNo            string                       `json:"requestNo" dc:"Stable assisted quote request number" eg:"AQR_20260811_0001"`
	CustomerNo           string                       `json:"customerNo" dc:"Target customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	CustomerCode         string                       `json:"customerCode" dc:"Customer-facing short code" eg:"10005"`
	CustomerDisplayName  string                       `json:"customerDisplayName" dc:"Safe customer display name" eg:"Example customer"`
	CustomerPrimaryPhone string                       `json:"customerPrimaryPhone" dc:"Safe primary phone summary" eg:"+8613800138005"`
	CurrencyCode         string                       `json:"currencyCode" dc:"Exact configured currency" eg:"CNY"`
	StatusCode           string                       `json:"statusCode" dc:"Pending approval, approved, rejected, or ordered" eg:"pending_approval"`
	ManualDiscountMinor  string                       `json:"manualDiscountMinor" dc:"Requested fixed discount in minor units" eg:"10000"`
	PaymentTermCode      string                       `json:"paymentTermCode" dc:"Prepayment deadline policy" eg:"extended_prepay"`
	PaymentDueSeconds    int                          `json:"paymentDueSeconds" dc:"Frozen order payment window in seconds" eg:"172800"`
	ContractPolicyCode   string                       `json:"contractPolicyCode" dc:"Order contract choice: inherit, none, or required" eg:"inherit"`
	ContractTemplateNo   string                       `json:"contractTemplateNo,omitempty" dc:"Selected enabled template when a contract is required" eg:"CNT_TEMPLATE_0001"`
	Reason               string                       `json:"reason" dc:"Staff business reason" eg:"Customer retention quotation request"`
	InternalNote         string                       `json:"internalNote" dc:"Internal staff note" eg:"Confirm current contract before approval"`
	RequestedBy          int                          `json:"requestedBy" dc:"Requesting staff user ID" eg:"1001"`
	DecisionCode         string                       `json:"decisionCode,omitempty" dc:"Independent decision, when completed" eg:"approved"`
	DecisionBy           int                          `json:"decisionBy,omitempty" dc:"Independent deciding staff user ID" eg:"1002"`
	DecisionReason       string                       `json:"decisionReason,omitempty" dc:"Independent decision reason" eg:"Pricing and customer scope verified"`
	DecisionAt           int64                        `json:"decisionAt,omitempty" dc:"Decision time as Unix milliseconds" eg:"1786406460000"`
	QuoteNo              string                       `json:"quoteNo,omitempty" dc:"Approved immutable quote number" eg:"QUO_20260811_0001"`
	OrderNo              string                       `json:"orderNo,omitempty" dc:"Created order number" eg:"ORD_20260811_0001"`
	OrderStatusCode      string                       `json:"orderStatusCode,omitempty" dc:"Current order status" eg:"pending_payment"`
	SubtotalMinor        string                       `json:"subtotalMinor" dc:"Fresh authoritative quote subtotal in minor units" eg:"200000"`
	DiscountMinor        string                       `json:"discountMinor" dc:"Approved discount in minor units" eg:"10000"`
	TotalAmountMinor     string                       `json:"totalAmountMinor" dc:"Fresh authoritative payable amount in minor units" eg:"190000"`
	ExpiresAt            int64                        `json:"expiresAt" dc:"Request expiry as Unix milliseconds" eg:"1786492800000"`
	CreatedAt            int64                        `json:"createdAt" dc:"Request creation time as Unix milliseconds" eg:"1786406400000"`
	Items                []*CommerceAssistedQuoteItem `json:"items" dc:"Bounded immutable request lines" eg:"[]"`
	CanApprove           bool                         `json:"canApprove" dc:"Whether an independent decision is currently workflow-eligible" eg:"true"`
	CanReject            bool                         `json:"canReject" dc:"Whether an independent rejection is currently workflow-eligible" eg:"true"`
	CanCheckout          bool                         `json:"canCheckout" dc:"Whether the approved quote is currently eligible for normal checkout" eg:"false"`
	Reused               bool                         `json:"reused" dc:"Whether an exact idempotent replay returned the committed result" eg:"false"`
}

type CommerceAssistedQuoteCreateReq struct {
	g.Meta              `path:"/admin/commerce/assisted-quotes" method:"post" tags:"IDC Commerce Management" summary:"Request a staff-assisted quote" dc:"Record bounded staff intent for independent approval; no paid, service, or fulfillment state can be supplied." permission:"idc:admin:commerce:assisted-quote:create"`
	CustomerNo          string                            `json:"customerNo" v:"required|max-length:40" dc:"Target customer number"`
	CurrencyCode        string                            `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Explicit configured currency"`
	ManualDiscountMinor string                            `json:"manualDiscountMinor" v:"required|regex:^([0-9]|[1-9][0-9]{0,15})$" dc:"Requested fixed discount over current authoritative price"`
	PaymentTermCode     string                            `json:"paymentTermCode" v:"required|in:standard_prepay,extended_prepay" dc:"Prepayment-only deadline policy"`
	PaymentDueSeconds   int                               `json:"paymentDueSeconds" dc:"Zero for standard policy; 86401 to 604800 for approved extension"`
	ContractPolicyCode  string                            `json:"contractPolicyCode" v:"required|in:inherit,none,required" dc:"Order contract choice"`
	ContractTemplateNo  string                            `json:"contractTemplateNo" v:"max-length:64" dc:"Required only when contractPolicyCode is required"`
	Reason              string                            `json:"reason" v:"required|length:10,1000" dc:"Business reason requiring independent approval"`
	InternalNote        string                            `json:"internalNote" v:"max-length:2000" dc:"Optional internal note"`
	ValiditySeconds     int                               `json:"validitySeconds" v:"required|min:900|max:604800" dc:"Approval-request validity window"`
	IdempotencyKey      string                            `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random action key stable across retries"`
	Items               []*CommerceAssistedQuoteItemInput `json:"items" v:"required" dc:"One to eight product purchase lines; collection bounds are enforced by the commerce service"`
}

type CommerceAssistedQuoteCreateRes struct {
	Request *CommerceAssistedQuote `json:"request" dc:"Committed pending-approval request"`
}

type CommerceAssistedQuoteListReq struct {
	Page       int `json:"page" v:"min:0" dc:"Optional one-based page number; cannot be combined with cursor" eg:"1"`
	g.Meta     `path:"/admin/commerce/assisted-quotes" method:"get" tags:"IDC Commerce Management" summary:"List staff-assisted quotes" dc:"Return one bounded encrypted-keyset page without exposing approval evidence digests." permission:"idc:admin:commerce:assisted-quote:list"`
	CustomerNo string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number"`
	StatusCode string `json:"statusCode" v:"in:,pending_approval,approved,rejected,ordered" dc:"Optional exact workflow status"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page"`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum requests returned"`
}

type CommerceAssistedQuoteListRes struct {
	Total      int                      `json:"total" dc:"Total records matching the current filters and authorized data scope" eg:"0"`
	Requests   []*CommerceAssistedQuote `json:"requests" dc:"Bounded assisted quote summaries"`
	NextCursor string                   `json:"nextCursor" dc:"Encrypted next-page cursor, or empty"`
}

type CommerceAssistedQuoteDetailReq struct {
	g.Meta    `path:"/admin/commerce/assisted-quotes/{requestNo}" method:"get" tags:"IDC Commerce Management" summary:"Get staff-assisted quote" dc:"Return one bounded request, quote, and order projection without approval evidence digests." permission:"idc:admin:commerce:assisted-quote:query"`
	RequestNo string `json:"requestNo" in:"path" v:"required|max-length:40" dc:"Stable assisted quote request number"`
}

type CommerceAssistedQuoteDetailRes struct {
	Request *CommerceAssistedQuote `json:"request" dc:"Assisted quote workflow projection"`
}

type CommerceAssistedQuoteDecisionReq struct {
	RequestNo      string `json:"requestNo" in:"path" v:"required|max-length:40" dc:"Stable assisted quote request number"`
	DecisionReason string `json:"decisionReason" v:"required|length:2,1000" dc:"Independent decision reason"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random action key stable across retries"`
}

type CommerceAssistedQuoteApprovalReq struct {
	g.Meta `path:"/admin/commerce/assisted-quotes/{requestNo}/approval" method:"put" tags:"IDC Commerce Management" summary:"Approve and freshly price staff-assisted quote" dc:"Require a different authorized staff member, derive evidence server-side, then revalidate customer, product, configuration, price, tax, and inventory before persisting an immutable quote." permission:"idc:admin:commerce:assisted-quote:approve"`
	CommerceAssistedQuoteDecisionReq
}

type CommerceAssistedQuoteApprovalRes struct {
	Request *CommerceAssistedQuote `json:"request" dc:"Approved request and fresh immutable quote"`
}

type CommerceAssistedQuoteRejectionReq struct {
	g.Meta `path:"/admin/commerce/assisted-quotes/{requestNo}/rejection" method:"put" tags:"IDC Commerce Management" summary:"Reject staff-assisted quote" dc:"Require a different authorized staff member, derive evidence server-side, and append an audited rejection without creating a quote or order." permission:"idc:admin:commerce:assisted-quote:reject"`
	CommerceAssistedQuoteDecisionReq
}

type CommerceAssistedQuoteRejectionRes struct {
	Request *CommerceAssistedQuote `json:"request" dc:"Rejected assisted quote request"`
}

type CommerceAssistedQuoteCheckoutReq struct {
	g.Meta         `path:"/admin/commerce/assisted-quotes/{requestNo}/checkout" method:"post" tags:"IDC Commerce Management" summary:"Checkout approved staff-assisted quote" dc:"Reuse normal order, invoice, inventory, fulfillment, outbox, and payment-preparation gates; no paid or active state can be supplied." permission:"idc:admin:commerce:assisted-quote:checkout"`
	RequestNo      string `json:"requestNo" in:"path" v:"required|max-length:40" dc:"Approved assisted quote request number"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random action key stable across retries"`
}

type CommerceAssistedQuoteCheckoutRes struct {
	Request *CommerceAssistedQuote `json:"request" dc:"Ordered assisted quote request"`
}
