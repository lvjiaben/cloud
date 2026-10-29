// Package v1 defines the authenticated customer commerce contract.
package v1

type CommerceCartItem struct {
	HasServerLogin                bool   `json:"hasServerLogin" dc:"Whether this independently configured server has private customer login choices and quantity must remain one" eg:"true"`
	ServerLoginToken              string `json:"serverLoginToken,omitempty" dc:"Opaque login choice token for this owned cart item; may be used only by the same customer and product version" eg:"PSL_0123456789abcdef"`
	ServerLoginHostname           string `json:"serverLoginHostname,omitempty" dc:"Customer-entered hostname, or empty for automatic generation" eg:"server.example"`
	ServerLoginPasswordConfigured bool   `json:"serverLoginPasswordConfigured" dc:"Whether a customer password is stored privately; the password is never returned" eg:"true"`

	Configuration          []*CommerceCartConfiguration `json:"configuration" dc:"Persisted priced choice labels and values owned by this cart; excludes credentials and private provider fields" eg:"[]"`
	ChargeTypeCode         string                       `json:"chargeTypeCode" dc:"Billing charge type from the selected published rule" eg:"recurring"`
	PeriodUnitCode         string                       `json:"periodUnitCode" dc:"Billing period unit from the selected published rule" eg:"month"`
	PeriodCount            int                          `json:"periodCount" dc:"Number of period units per billing cycle" eg:"1"`
	ConfigurationReference string                       `json:"configurationReference" dc:"Opaque selected-configuration reference retained through cart merge; never configuration values or an ownership credential" eg:"CFS_01J2ABCDEF"`
	CartItemNo             string                       `json:"cartItemNo" dc:"Stable cart item number"`
	ProductNameZhCN        string                       `json:"productNameZhCN" dc:"Simplified Chinese product name"`
	ProductNameEnUS        string                       `json:"productNameEnUS" dc:"English product name"`
	PublicSlug             string                       `json:"publicSlug,omitempty" dc:"Public product slug used only to open governed customer configuration"`
	ConfigurationRequired  bool                         `json:"configurationRequired" dc:"Whether checkout requires governed customer configuration"`
	ConfigurationFrozen    bool                         `json:"configurationFrozen" dc:"Whether the latest server-authoritative quote item has an immutable governed configuration" eg:"true"`
	ConfigurationRevision  string                       `json:"configurationRevision,omitempty" dc:"Opaque governed configuration revision frozen for the latest quote item, or empty" eg:"Q1JfYWJj"`
	Quantity               int                          `json:"quantity" dc:"Number of product units" eg:"1"`
	PurchaseCycles         int                          `json:"purchaseCycles" dc:"Number of billing cycles" eg:"1"`
	StatusCode             string                       `json:"statusCode" dc:"Current cart item status" eg:"active"`
	IssueCode              string                       `json:"issueCode" dc:"Stable availability issue code, or empty"`
	RepairAction           string                       `json:"repairAction" dc:"Suggested customer repair action, or empty"`
	Version                int64                        `json:"version" dc:"Cart item optimistic concurrency version" eg:"1"`
}

// CommerceCartConfiguration is a customer-safe persisted choice projection.
type CommerceCartConfiguration struct {
	LabelZhCN string `json:"labelZhCN" dc:"Chinese field label" eg:"CPU"`
	LabelEnUS string `json:"labelEnUS" dc:"English field label" eg:"CPU"`
	ValueZhCN string `json:"valueZhCN" dc:"Chinese selected value label" eg:"4核"`
	ValueEnUS string `json:"valueEnUS" dc:"English selected value label" eg:"4 cores"`
}

type CommerceCart struct {
	CartNo       string              `json:"cartNo" dc:"Stable account cart number"`
	CurrencyCode string              `json:"currencyCode" dc:"Exact cart currency" eg:"CNY"`
	Version      int64               `json:"version" dc:"Cart optimistic concurrency version" eg:"1"`
	Items        []*CommerceCartItem `json:"items" dc:"Bounded cart items"`
	LatestQuote  *CommerceQuote      `json:"latestQuote,omitempty" dc:"Latest unexpired server-authoritative quote for this cart, including configuration freeze state" eg:"{}"`
}

type CommerceQuoteAdjustment struct {
	TypeCode        string `json:"typeCode" dc:"Stable adjustment type" eg:"coupon"`
	SourceReference string `json:"sourceReference" dc:"Customer-visible adjustment reference"`
	AmountMinor     string `json:"amountMinor" dc:"Exact decimal integer minor units" eg:"100"`
}

type CommerceQuoteItem struct {
	QuoteItemNo           string                     `json:"quoteItemNo" dc:"Stable immutable quote item number"`
	CartItemNo            string                     `json:"cartItemNo" dc:"Source cart item number"`
	ProductNameZhCN       string                     `json:"productNameZhCN" dc:"Simplified Chinese frozen product name"`
	ProductNameEnUS       string                     `json:"productNameEnUS" dc:"English frozen product name"`
	PublicSlug            string                     `json:"publicSlug,omitempty" dc:"Public product slug used only to open governed customer configuration"`
	ConfigurationRequired bool                       `json:"configurationRequired" dc:"Whether this exact quote item requires a frozen governed configuration"`
	ConfigurationFrozen   bool                       `json:"configurationFrozen" dc:"Whether this exact quote item has an immutable governed configuration" eg:"true"`
	ConfigurationRevision string                     `json:"configurationRevision,omitempty" dc:"Opaque governed configuration revision frozen for this quote item, or empty" eg:"Q1JfYWJj"`
	Quantity              int                        `json:"quantity" dc:"Number of product units"`
	PurchaseCycles        int                        `json:"purchaseCycles" dc:"Number of billing cycles"`
	FulfillmentModeCode   string                     `json:"fulfillmentModeCode" dc:"Automatic or manual fulfillment decision"`
	RiskDecisionCode      string                     `json:"riskDecisionCode" dc:"Frozen risk decision"`
	InventoryDecisionCode string                     `json:"inventoryDecisionCode" dc:"Frozen inventory decision"`
	SetupSubtotalMinor    string                     `json:"setupSubtotalMinor" dc:"Exact setup subtotal minor units"`
	PeriodSubtotalMinor   string                     `json:"periodSubtotalMinor" dc:"Exact recurring period subtotal minor units"`
	SubtotalMinor         string                     `json:"subtotalMinor" dc:"Exact subtotal minor units"`
	DiscountMinor         string                     `json:"discountMinor" dc:"Exact discount minor units"`
	NetAmountMinor        string                     `json:"netAmountMinor" dc:"Exact tax-exclusive minor units"`
	TaxAmountMinor        string                     `json:"taxAmountMinor" dc:"Exact tax minor units"`
	TotalAmountMinor      string                     `json:"totalAmountMinor" dc:"Exact payable minor units"`
	Adjustments           []*CommerceQuoteAdjustment `json:"adjustments" dc:"Bounded deterministic adjustment explanations"`
}

type CommerceQuote struct {
	QuoteNo          string               `json:"quoteNo" dc:"Stable immutable quote number"`
	CartNo           string               `json:"cartNo" dc:"Source account cart number"`
	CurrencyCode     string               `json:"currencyCode" dc:"Exact quote currency"`
	CartVersion      int64                `json:"cartVersion" dc:"Cart version priced by this quote"`
	SubtotalMinor    string               `json:"subtotalMinor" dc:"Exact subtotal minor units"`
	DiscountMinor    string               `json:"discountMinor" dc:"Exact discount minor units"`
	NetAmountMinor   string               `json:"netAmountMinor" dc:"Exact tax-exclusive minor units"`
	TaxAmountMinor   string               `json:"taxAmountMinor" dc:"Exact tax minor units"`
	TotalAmountMinor string               `json:"totalAmountMinor" dc:"Exact payable minor units"`
	IssuedAt         int64                `json:"issuedAt" dc:"Quote issue time as Unix milliseconds"`
	ExpiresAt        int64                `json:"expiresAt" dc:"Quote expiry time as Unix milliseconds"`
	Items            []*CommerceQuoteItem `json:"items" dc:"Bounded immutable quote items"`
}

type CommerceCheckout struct {
	OrderNo          string `json:"orderNo" dc:"Created or safely replayed order number"`
	InvoiceNo        string `json:"invoiceNo" dc:"Created receivable invoice number"`
	QuoteNo          string `json:"quoteNo" dc:"Consumed immutable quote number"`
	StatusCode       string `json:"statusCode" dc:"Current order status"`
	CurrencyCode     string `json:"currencyCode" dc:"Exact order currency"`
	ItemCount        int    `json:"itemCount" dc:"Number of order items"`
	TotalAmountMinor string `json:"totalAmountMinor" dc:"Exact payable minor units"`

	PaymentDueAt int64 `json:"paymentDueAt" dc:"Frozen payment deadline as Unix milliseconds"`
	Reused       bool  `json:"reused" dc:"Whether exact idempotent replay returned the first order"`
}

type CommercePaymentPreparation struct {
	ContextNo          string `json:"contextNo" dc:"Stable resumable payment preparation number"`
	OrderNo            string `json:"orderNo" dc:"Customer-owned order number"`
	InvoiceNo          string `json:"invoiceNo" dc:"Receivable invoice number"`
	CustomerTypeCode   string `json:"customerTypeCode" dc:"Individual or enterprise customer type"`
	StatusCode         string `json:"statusCode" dc:"Current payment preparation status"`
	ReasonCode         string `json:"reasonCode" dc:"Stable customer-action reason"`
	IdentityStatusCode string `json:"identityStatusCode" dc:"Current authoritative identity status"`
	CurrencyCode       string `json:"currencyCode" dc:"Exact invoice currency"`
	AmountDueMinor     string `json:"amountDueMinor" dc:"Exact amount still due in minor units"`

	PaymentDueAt    int64 `json:"paymentDueAt" dc:"Frozen payment deadline as Unix milliseconds"`
	LastEvaluatedAt int64 `json:"lastEvaluatedAt" dc:"Latest authoritative evaluation time as Unix milliseconds"`
	Ready           bool  `json:"ready" dc:"Whether the current evaluation permits payment attempt creation"`
	Reused          bool  `json:"reused" dc:"Whether the existing preparation context was reused"`
}

type CommerceOrderCancellation struct {
	CancellationNo string `json:"cancellationNo" dc:"Stable cancellation number"`
	OrderNo        string `json:"orderNo" dc:"Customer-owned order number"`
	StatusCode     string `json:"statusCode" dc:"Current cancellation status"`
	ReasonCode     string `json:"reasonCode" dc:"Stable cancellation reason"`
	RequestedAt    int64  `json:"requestedAt" dc:"Request time as Unix milliseconds"`
	CompletedAt    int64  `json:"completedAt,omitempty" dc:"Completion time as Unix milliseconds, or zero"`
	Reused         bool   `json:"reused" dc:"Whether exact idempotent replay returned the first cancellation"`
}

type CommerceOrderSummary struct {
	OrderNo              string `json:"orderNo" dc:"Stable order number"`
	CustomerNo           string `json:"customerNo,omitempty" dc:"Owning customer number, returned only to staff"`
	CustomerCode         string `json:"customerCode,omitempty" dc:"Short numeric customer number returned only to staff"`
	CustomerDisplayName  string `json:"customerDisplayName,omitempty" dc:"Display-safe customer name returned only to staff"`
	CustomerPrimaryPhone string `json:"customerPrimaryPhone,omitempty" dc:"Primary customer phone returned only to staff"`
	CustomerTypeCode     string `json:"customerTypeCode,omitempty" dc:"Owning customer type, returned only to staff" eg:"enterprise"`
	ProductSummary       string `json:"productSummary,omitempty" dc:"Display-safe localized product summary aggregated for this order"`
	IPAddress            string `json:"ipAddress,omitempty" dc:"Display-safe current service IP when the resource owner has published one"`
	PaymentMethodCode    string `json:"paymentMethodCode,omitempty" dc:"Payment source type recorded by the billing ledger"`
	Remark               string `json:"remark,omitempty" dc:"Display-safe staff order remark when one exists"`
	StatusCode           string `json:"statusCode" dc:"Current order status"`
	RiskDecisionCode     string `json:"riskDecisionCode" dc:"Current risk decision"`
	CurrencyCode         string `json:"currencyCode" dc:"Exact order currency"`
	ItemCount            int    `json:"itemCount" dc:"Number of order items"`
	TotalAmountMinor     string `json:"totalAmountMinor" dc:"Exact total minor units"`

	PaymentDueAt      int64  `json:"paymentDueAt,omitempty" dc:"Payment deadline as Unix milliseconds, or zero"`
	CreatedAt         int64  `json:"createdAt" dc:"Order creation time as Unix milliseconds"`
	CanCancel         bool   `json:"canCancel" dc:"Whether customer cancellation may be requested"`
	CanPreparePayment bool   `json:"canPreparePayment" dc:"Whether payment preparation may be evaluated"`
	AssistedRequestNo string `json:"assistedRequestNo,omitempty" dc:"Originating assisted quote request, returned only to staff"`
	PaymentTermCode   string `json:"paymentTermCode" dc:"Frozen prepayment deadline policy"`
}

type CommerceOrderInvoice struct {
	InvoiceNo             string `json:"invoiceNo" dc:"Stable invoice number"`
	StatusCode            string `json:"statusCode" dc:"Current invoice status"`
	CurrencyCode          string `json:"currencyCode" dc:"Exact invoice currency"`
	SubtotalMinor         string `json:"subtotalMinor" dc:"Exact subtotal minor units"`
	DiscountMinor         string `json:"discountMinor" dc:"Exact discount minor units"`
	NetAmountMinor        string `json:"netAmountMinor" dc:"Exact net minor units"`
	TaxAmountMinor        string `json:"taxAmountMinor" dc:"Exact tax minor units"`
	TotalAmountMinor      string `json:"totalAmountMinor" dc:"Exact total minor units"`
	PaidAmountMinor       string `json:"paidAmountMinor" dc:"Exact paid minor units"`
	RefundableAmountMinor string `json:"refundableAmountMinor" dc:"Exact refundable minor units"`
	RefundedAmountMinor   string `json:"refundedAmountMinor" dc:"Exact refunded minor units"`
	PaymentDueAt          int64  `json:"paymentDueAt,omitempty" dc:"Payment deadline as Unix milliseconds, or zero"`
	IssuedAt              int64  `json:"issuedAt,omitempty" dc:"Issue time as Unix milliseconds, or zero"`
	SettledAt             int64  `json:"settledAt,omitempty" dc:"Settlement time as Unix milliseconds, or zero"`
	CancelledAt           int64  `json:"cancelledAt,omitempty" dc:"Cancellation time as Unix milliseconds, or zero"`
}

type CommerceOrderItem struct {
	OrderItemNo     string `json:"orderItemNo" dc:"Stable order-item number"`
	LineNumber      int    `json:"lineNumber" dc:"Stable line sequence"`
	ProductNameZhCN string `json:"productNameZhCN" dc:"Frozen Chinese product name"`
	ProductNameEnUS string `json:"productNameEnUS" dc:"Frozen English product name"`
	Quantity        int    `json:"quantity" dc:"Number of product units"`
	PurchaseCycles  int    `json:"purchaseCycles" dc:"Purchased billing cycles"`
	ChargeTypeCode  string `json:"chargeTypeCode" dc:"Frozen charge type"`
	PeriodUnitCode  string `json:"periodUnitCode" dc:"Frozen billing period unit"`
	PeriodCount     int    `json:"periodCount" dc:"Frozen billing period count"`

	FulfillmentModeCode   string `json:"fulfillmentModeCode" dc:"Fulfillment decision"`
	RiskDecisionCode      string `json:"riskDecisionCode" dc:"Risk decision"`
	InventoryDecisionCode string `json:"inventoryDecisionCode" dc:"Inventory decision"`
	SetupSubtotalMinor    string `json:"setupSubtotalMinor" dc:"Exact setup subtotal minor units"`
	PeriodSubtotalMinor   string `json:"periodSubtotalMinor" dc:"Exact recurring subtotal minor units"`
	SubtotalMinor         string `json:"subtotalMinor" dc:"Exact subtotal minor units"`
	DiscountMinor         string `json:"discountMinor" dc:"Exact discount minor units"`
	NetAmountMinor        string `json:"netAmountMinor" dc:"Exact net minor units"`
	TaxAmountMinor        string `json:"taxAmountMinor" dc:"Exact tax minor units"`
	TotalAmountMinor      string `json:"totalAmountMinor" dc:"Exact total minor units"`
}

type CommerceDelivery struct {
	ProductNameZhCN string `json:"productNameZhCN" dc:"Frozen Simplified Chinese product name" eg:"SSL Certificate"`
	ProductNameEnUS string `json:"productNameEnUS" dc:"Frozen English product name" eg:"SSL Certificate"`
	Quantity        int    `json:"quantity" dc:"Delivered product quantity" eg:"1"`
	StatusCode      string `json:"statusCode" dc:"Customer-safe status: awaiting_payment, awaiting_review, processing, manual_processing (staff attention required), completed, not_completed, or cancelled" eg:"processing"`
	OngoingService  bool   `json:"ongoingService" dc:"Whether the delivered product includes ongoing service management" eg:"true"`
	StartedAt       int64  `json:"startedAt,omitempty" dc:"Processing start time as Unix milliseconds, or zero" eg:"1787803200000"`
	CompletedAt     int64  `json:"completedAt,omitempty" dc:"Completion time as Unix milliseconds, or zero" eg:"1787806800000"`
	UpdatedAt       int64  `json:"updatedAt" dc:"Last authoritative update time as Unix milliseconds" eg:"1787806800000"`
}

type CommerceServiceEntitlement struct {
	ManagementPreparation string `json:"managementPreparation,omitempty" dc:"Optional local service management readiness: preparing, ready, or needs_attention; absent for historical services without a notification; does not change delivery, lifecycle or billing" eg:"preparing"`
	EntitlementOwnerID    string `json:"entitlementOwnerId,omitempty" dc:"Frozen owner plugin for service management navigation" eg:"idc-resource"`
	EntitlementNo         string `json:"entitlementNo" dc:"Stable customer-owned service entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
	OrderItemNo           string `json:"orderItemNo" dc:"Stable owning order-item number" eg:"ord_item_01J2ABCDEF"`
	ProductNameZhCN       string `json:"productNameZhCN" dc:"Frozen Simplified Chinese product name" eg:"SSL Certificate"`
	ProductNameEnUS       string `json:"productNameEnUS" dc:"Frozen English product name" eg:"SSL Certificate"`
	StatusCode            string `json:"statusCode" dc:"Customer-safe service status: active, suspended, expired, or terminated" eg:"active"`
	StartedAt             int64  `json:"startedAt" dc:"Service start time as Unix milliseconds" eg:"1787806800000"`
	ExpiresAt             int64  `json:"expiresAt,omitempty" dc:"Service expiry time as Unix milliseconds, or zero" eg:"1819342800000"`
}

type CommerceOrderReview struct {
	ReviewNo       string `json:"reviewNo" dc:"Stable review number"`
	ReviewTypeCode string `json:"reviewTypeCode" dc:"Review type"`
	StatusCode     string `json:"statusCode" dc:"Current review status"`
	ReasonCode     string `json:"reasonCode" dc:"Stable review reason"`
	RequestedAt    int64  `json:"requestedAt,omitempty" dc:"Request time as Unix milliseconds, or zero"`
	ReviewedAt     int64  `json:"reviewedAt,omitempty" dc:"Decision time as Unix milliseconds, or zero"`
}

type CommerceOrderCancellationDetail struct {
	CancellationNo            string `json:"cancellationNo" dc:"Stable cancellation number"`
	SourceCode                string `json:"sourceCode" dc:"Cancellation source"`
	ReasonCode                string `json:"reasonCode" dc:"Cancellation reason"`
	StatusCode                string `json:"statusCode" dc:"Current cancellation status"`
	PaymentClosureOutcomeCode string `json:"paymentClosureOutcomeCode,omitempty" dc:"Payment closure outcome, when known"`
	RequestedAt               int64  `json:"requestedAt,omitempty" dc:"Request time as Unix milliseconds, or zero"`
	CompletedAt               int64  `json:"completedAt,omitempty" dc:"Completion time as Unix milliseconds, or zero"`
}

type CommerceOrderPaymentPreparation struct {
	ContextNo          string `json:"contextNo" dc:"Recoverable payment-preparation context"`
	StatusCode         string `json:"statusCode" dc:"Current preparation status"`
	ReasonCode         string `json:"reasonCode" dc:"Current preparation reason"`
	IdentityStatusCode string `json:"identityStatusCode" dc:"Current identity decision"`
	LastEvaluatedAt    int64  `json:"lastEvaluatedAt,omitempty" dc:"Last evaluation time as Unix milliseconds, or zero"`
	ReadyAt            int64  `json:"readyAt,omitempty" dc:"Ready time as Unix milliseconds, or zero"`
	ExpiresAt          int64  `json:"expiresAt,omitempty" dc:"Context expiry as Unix milliseconds, or zero"`
}

type CommerceOrderDetail struct {
	Summary             *CommerceOrderSummary            `json:"summary" dc:"Order summary and server-derived actions"`
	Invoice             *CommerceOrderInvoice            `json:"invoice,omitempty" dc:"Invoice facts, when created"`
	Items               []*CommerceOrderItem             `json:"items" dc:"Bounded frozen order lines"`
	Deliveries          []*CommerceDelivery              `json:"deliveries" dc:"One customer-safe delivery record for each order item" eg:"[]"`
	ServiceEntitlements []*CommerceServiceEntitlement    `json:"serviceEntitlements" dc:"Ongoing service rights created by completed deliveries" eg:"[]"`
	Cancellation        *CommerceOrderCancellationDetail `json:"cancellation,omitempty" dc:"Cancellation facts, when requested"`
	Reviews             []*CommerceOrderReview           `json:"reviews" dc:"Bounded review facts"`
	PaymentPreparation  *CommerceOrderPaymentPreparation `json:"paymentPreparation,omitempty" dc:"Recoverable payment context, when prepared"`
	CanApproveRisk      bool                             `json:"canApproveRisk" dc:"Whether the order is eligible for staff risk approval"`
	CanRejectRisk       bool                             `json:"canRejectRisk" dc:"Whether the order is eligible for staff risk rejection"`
}
