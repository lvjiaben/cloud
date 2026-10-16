// Package v1 defines staff administration contracts for downstream channels.
package v1

import "github.com/gogf/gf/v2/frame/g"

// PageRequest bounds numeric pagination at the HTTP boundary.
type PageRequest struct {
	Page     int `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number" eg:"1"`
	PageSize int `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Rows per page" eg:"10"`
}

// PageMetadata describes the exact visible collection rather than the returned slice.
type PageMetadata struct {
	Page     int `json:"page" dc:"Page number" eg:"1"`
	PageSize int `json:"pageSize" dc:"Rows per page" eg:"10"`
	Total    int `json:"total" dc:"Total visible records" eg:"23"`
}

type ChannelSummary struct {
	ChannelId       string `json:"channelId"`
	CustomerNo      string `json:"customerNo"`
	DisplayName     string `json:"displayName"`
	StatusCode      string `json:"statusCode"`
	NewSalesEnabled bool   `json:"newSalesEnabled"`
	CreatedAt       int64  `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1788163200000"`
	UpdatedAt       int64  `json:"updatedAt" dc:"Last update time as a Unix timestamp in milliseconds" eg:"1788166800000"`
}
type ListChannelsReq struct {
	g.Meta `path:"/admin/channel/accounts" method:"get" tags:"Downstream Channel" summary:"List channel accounts" permission:"idc:admin:channel:token"`
	PageRequest
	Keyword string `json:"keyword" v:"max-length:160" dc:"Channel name or identifier search" eg:"partner"`
}
type ListChannelsRes struct {
	PageMetadata
	List []*ChannelSummary `json:"list"`
}
type SaveChannelReq struct {
	g.Meta      `path:"/admin/channel/accounts" method:"post" tags:"Downstream Channel" summary:"Create or update a channel account" permission:"idc:admin:channel:token"`
	ChannelId   string `json:"channelId" v:"required|length:1,80"`
	CustomerNo  string `json:"customerNo" v:"required|length:1,40"`
	DisplayName string `json:"displayName" v:"required|length:1,160"`
}
type SaveChannelRes struct {
	Channel *ChannelSummary `json:"channel"`
}
type SetChannelStatusReq struct {
	g.Meta    `path:"/admin/channel/accounts/{channelId}/status" method:"post" tags:"Downstream Channel" summary:"Approve, pause, resume or close a channel" permission:"idc:admin:channel:token"`
	ChannelId string `json:"channelId" v:"required|length:1,80"`
	Action    string `json:"action" v:"required|in:approve,pause,resume,close"`
}
type SetChannelStatusRes struct {
	Channel *ChannelSummary `json:"channel"`
}

type PurchaseValue struct {
	FieldToken          string   `json:"fieldToken"`
	Type                string   `json:"type"`
	TextValue           *string  `json:"textValue"`
	IntegerValue        *string  `json:"integerValue"`
	DecimalValue        *string  `json:"decimalValue"`
	BooleanValue        *bool    `json:"booleanValue"`
	DateValue           *string  `json:"dateValue"`
	DateTimeValue       *string  `json:"dateTimeValue"`
	OptionTokens        []string `json:"optionTokens"`
	TreeOptionTokens    []string `json:"treeOptionTokens"`
	AddressOptionTokens []string `json:"addressOptionTokens"`
	FileTokens          []string `json:"fileTokens"`
}
type PublicationSummary struct {
	ChannelId             string `json:"channelId"`
	ProductVersionId      int64  `json:"productVersionId"`
	Currency              string `json:"currency"`
	PriceMinor            int64  `json:"priceMinor"`
	Stock                 int    `json:"stock"`
	Status                string `json:"status"`
	PublicSlug            string `json:"publicSlug"`
	ConfigurationRevision string `json:"configurationRevision"`
	BillingTermToken      string `json:"billingTermToken"`
	PurchaseCycles        int    `json:"purchaseCycles"`
	Quantity              int    `json:"quantity"`
}
type ListPublicationsReq struct {
	PageRequest
	g.Meta    `path:"/admin/channel/publications" method:"get" tags:"Downstream Channel" summary:"List channel publications" permission:"idc:admin:channel:token"`
	ChannelId string `json:"channelId" v:"required|length:1,80"`
}
type ListPublicationsRes struct {
	PageMetadata
	List []*PublicationSummary `json:"list"`
}
type SavePublicationReq struct {
	g.Meta                `path:"/admin/channel/publications" method:"post" tags:"Downstream Channel" summary:"Publish an authoritative core purchase configuration" permission:"idc:admin:channel:token"`
	ChannelId             string          `json:"channelId" v:"required|length:1,80"`
	ProductVersionId      int64           `json:"productVersionId" v:"required"`
	Status                string          `json:"status" v:"required|in:published,paused"`
	Stock                 int             `json:"stock" v:"min:0"`
	PublicSlug            string          `json:"publicSlug" v:"required"`
	ConfigurationRevision string          `json:"configurationRevision" v:"required"`
	BillingTermToken      string          `json:"billingTermToken" v:"required"`
	Currency              string          `json:"currency" v:"required|length:3,3"`
	PurchaseCycles        int             `json:"purchaseCycles" v:"required|min:1|max:120"`
	Quantity              int             `json:"quantity" v:"required|min:1|max:1000"`
	Values                []PurchaseValue `json:"values"`
}
type SavePublicationRes struct {
	Publication *PublicationSummary `json:"publication"`
}

type RequestAudit struct {
	Id          int64  `json:"id"`
	ChannelId   string `json:"channelId"`
	RequestId   string `json:"requestId"`
	Operation   string `json:"operation"`
	Status      string `json:"status"`
	ResultCode  string `json:"resultCode"`
	CreatedAt   int64  `json:"createdAt" dc:"Request creation time as a Unix timestamp in milliseconds" eg:"1788163200000"`
	CompletedAt *int64 `json:"completedAt" dc:"Optional completion time as a Unix timestamp in milliseconds" eg:"1788163201000"`
}
type ListRequestAuditReq struct {
	PageRequest
	g.Meta    `path:"/admin/channel/requests" method:"get" tags:"Downstream Channel" summary:"List bounded channel request audit" permission:"idc:admin:channel:token"`
	ChannelId string `json:"channelId" v:"required|length:1,80"`
}
type ListRequestAuditRes struct {
	PageMetadata
	List []*RequestAudit `json:"list"`
}
type HealthReq struct {
	g.Meta    `path:"/admin/channel/health" method:"get" tags:"Downstream Channel" summary:"Get channel health summary" permission:"idc:admin:channel:token"`
	ChannelId string `json:"channelId" v:"required|length:1,80"`
}
type HealthRes struct {
	ChannelId           string `json:"channelId"`
	StatusCode          string `json:"statusCode"`
	NewSalesEnabled     bool   `json:"newSalesEnabled"`
	ActiveTokenCount    int    `json:"activeTokenCount"`
	PublicationCount    int    `json:"publicationCount"`
	PendingWebhookCount int    `json:"pendingWebhookCount"`
	LastRequestAt       *int64 `json:"lastRequestAt" dc:"Last request time as a Unix timestamp in milliseconds" eg:"1788163200000"`
}
type FinanceReq struct {
	g.Meta            `path:"/admin/channel/finance" method:"get" tags:"Downstream Channel" summary:"Get channel finance reconciliation" permission:"idc:admin:channel:token"`
	ChannelId         string `json:"channelId" v:"required|length:1,80"`
	InvoicePage       int    `json:"invoicePage" d:"1" v:"min:1|max:1000000" dc:"Invoice page number" eg:"1"`
	InvoicePageSize   int    `json:"invoicePageSize" d:"10" v:"min:1|max:100" dc:"Invoices per page" eg:"10"`
	StatementPage     int    `json:"statementPage" d:"1" v:"min:1|max:1000000" dc:"Credit statement page number" eg:"1"`
	StatementPageSize int    `json:"statementPageSize" d:"10" v:"min:1|max:100" dc:"Credit statements per page" eg:"10"`
	RepaymentPage     int    `json:"repaymentPage" d:"1" v:"min:1|max:1000000" dc:"Credit repayment page number" eg:"1"`
	RepaymentPageSize int    `json:"repaymentPageSize" d:"10" v:"min:1|max:100" dc:"Credit repayments per page" eg:"10"`
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
	InvoiceTotal     int                `json:"invoiceTotal" dc:"Total visible invoices" eg:"23"`
	StatementTotal   int                `json:"statementTotal" dc:"Total visible credit statements" eg:"23"`
	RepaymentTotal   int                `json:"repaymentTotal" dc:"Total visible credit repayments" eg:"23"`
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

type CapabilitiesReq struct {
	g.Meta `path:"/admin/channel/capabilities" method:"get" tags:"Downstream Channel" summary:"List verified channel capabilities for staff" permission:"idc:admin:channel:token"`
}
type CapabilitiesRes struct {
	GenericAPI      bool              `json:"genericApi"`
	WebhookDelivery bool              `json:"webhookDelivery"`
	Adapters        map[string]string `json:"adapters"`
	Operations      map[string]string `json:"operations"`
}

type CatalogReq struct {
	PageRequest
	g.Meta    `path:"/admin/channel/catalog" method:"get" tags:"Downstream Channel" summary:"List sanitized channel catalog for staff" permission:"idc:admin:channel:token"`
	ChannelId string `json:"channelId" v:"required|length:1,80"`
}
type CatalogItem struct {
	ProductVersionId int64  `json:"productVersionId"`
	Currency         string `json:"currency"`
	Price            string `json:"price"`
	Stock            int    `json:"stock"`
}
type CatalogRes struct {
	PageMetadata
	List []*CatalogItem `json:"list"`
}

type ListTokensReq struct {
	PageRequest
	g.Meta    `path:"/admin/channel/tokens" method:"get" tags:"Downstream Channel" summary:"List channel token metadata" permission:"idc:admin:channel:token"`
	ChannelId string `json:"channelId"`
}
type TokenSummary struct {
	Id          int64    `json:"id"`
	ChannelId   string   `json:"channelId"`
	Scopes      []string `json:"scopes"`
	SourceCIDRs []string `json:"sourceCidrs"`
	ExpiresAt   *int64   `json:"expiresAt" dc:"Optional token expiry as a Unix timestamp in milliseconds" eg:"1798761600000"`
	RevokedAt   *int64   `json:"revokedAt" dc:"Revocation time as a Unix timestamp in milliseconds when revoked" eg:"1788166800000"`
	CreatedAt   int64    `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1788163200000"`
}
type ListTokensRes struct {
	PageMetadata
	List []*TokenSummary `json:"list"`
}

type CreateTokenReq struct {
	g.Meta      `path:"/admin/channel/tokens" method:"post" tags:"Downstream Channel" summary:"Create one-time channel token" permission:"idc:admin:channel:token"`
	ChannelId   string   `json:"channelId" v:"max-length:80"`
	CustomerNo  string   `json:"customerNo" v:"max-length:40" dc:"Customer receiving API access; required when creating access" eg:"cus_example"`
	DisplayName string   `json:"displayName" v:"max-length:160" dc:"Human-readable connection name" eg:"Partner shop"`
	Scopes      []string `json:"scopes"`
	SourceCIDRs []string `json:"sourceCidrs"`
	ExpiresAt   *int64   `json:"expiresAt" dc:"Optional future expiry as a Unix timestamp in milliseconds" eg:"1798761600000"`
}
type CreateTokenRes struct {
	AppID     string `json:"appId" dc:"Application identifier paired with the one-time secret" eg:"app_example"`
	Id        int64  `json:"id"`
	Token     string `json:"token"`
	ExpiresAt *int64 `json:"expiresAt" dc:"Optional token expiry as a Unix timestamp in milliseconds" eg:"1798761600000"`
}
type RevokeTokenReq struct {
	g.Meta `path:"/admin/channel/tokens/{id}" method:"delete" tags:"Downstream Channel" summary:"Revoke channel token" permission:"idc:admin:channel:token"`
	Id     int64 `json:"id" v:"required"`
}
type RevokeTokenRes struct {
	Id      int64 `json:"id"`
	Revoked bool  `json:"revoked"`
}
