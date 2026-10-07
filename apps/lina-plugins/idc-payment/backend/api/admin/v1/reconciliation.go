// This file declares paged reconciliation reads and controlled two-person
// supplements. Numeric pages keep the same channel and batch scope as each total.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ReconciliationHealthReq struct {
	g.Meta   `path:"/admin/payment-reconciliation/channels" method:"get" tags:"IDC Payment Reconciliation" summary:"List payment-channel reconciliation health" dc:"Return one bounded channel health page and its exact visible total, preserving explicit unknown states." permission:"idc:admin:payment:reconciliation:view"`
	Page     int `json:"page" d:"1" v:"between:1,1000000" dc:"One-based page number" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"between:1,100" dc:"Number of rows per page from 1 to 100" eg:"20"`
}
type ReconciliationHealthItem struct {
	ChannelNo               string `json:"channelNo" dc:"Local payment channel number" eg:"channel_alipay"`
	DisplayName             string `json:"displayName" dc:"Channel display name" eg:"Alipay"`
	ProviderCode            string `json:"providerCode" dc:"Provider code" eg:"alipay"`
	CapabilityCode          string `json:"capabilityCode" dc:"Reconciliation capability: statement, authoritative_query, unavailable" eg:"statement"`
	HealthCode              string `json:"healthCode" dc:"Latest health: healthy, differences, failed, unavailable, unknown" eg:"healthy"`
	LatestBatchNo           string `json:"latestBatchNo" dc:"Latest daily batch number; empty when never run" eg:"prb_123"`
	FailureCode             string `json:"failureCode" dc:"Fixed latest failure code; empty after success" eg:"SOURCE_UNAVAILABLE"`
	LatestBusinessDate      *int64 `json:"latestBusinessDate" dc:"Latest merchant business date in Unix milliseconds; null when never run" eg:"1787155200000"`
	UnresolvedCount         int    `json:"unresolvedCount" dc:"Unresolved difference count" eg:"2"`
	ConsecutiveFailureCount int    `json:"consecutiveFailureCount" dc:"Consecutive failed daily run count" eg:"1"`
	ResultUnknown           bool   `json:"resultUnknown" dc:"Whether the latest reconciliation result is unknown" eg:"false"`
}
type ReconciliationHealthRes struct {
	Channels []*ReconciliationHealthItem `json:"channels" dc:"Configured channel reconciliation health list" eg:"[]"`
	Total    int                         `json:"total" dc:"Exact number of visible matching records" eg:"123"`
}

type ReconciliationBatchListReq struct {
	g.Meta    `path:"/admin/payment-reconciliation/batches" method:"get" tags:"IDC Payment Reconciliation" summary:"List bounded daily reconciliation batches" dc:"Return one bounded batch page and its exact total, optionally filtered by one exact local channel." permission:"idc:admin:payment:reconciliation:view"`
	ChannelNo string `json:"channelNo" in:"query" v:"max-length:64" dc:"Exact channel filter; empty means all visible channels" eg:"channel_alipay"`
	Page      int    `json:"page" d:"1" v:"between:1,1000000" dc:"One-based page number" eg:"1"`
	PageSize  int    `json:"pageSize" d:"20" v:"between:1,100" dc:"Number of rows per page from 1 to 100" eg:"20"`
}
type ReconciliationBatchItem struct {
	BatchNo         string `json:"batchNo" dc:"Immutable reconciliation batch number" eg:"prb_123"`
	ChannelNo       string `json:"channelNo" dc:"Local payment channel number" eg:"channel_alipay"`
	CapabilityCode  string `json:"capabilityCode" dc:"Used capability: statement, authoritative_query, unavailable" eg:"statement"`
	StatusCode      string `json:"statusCode" dc:"Batch status: processing, completed, failed" eg:"completed"`
	FailureCode     string `json:"failureCode" dc:"Fixed failure code; empty after success" eg:"SOURCE_UNAVAILABLE"`
	BusinessDate    int64  `json:"businessDate" dc:"Merchant business date in Unix milliseconds" eg:"1787155200000"`
	TotalCount      int    `json:"totalCount" dc:"Compared local and provider record count" eg:"20"`
	DifferenceCount int    `json:"differenceCount" dc:"Detected difference count" eg:"2"`
	PaymentCount    int    `json:"paymentCount" dc:"Compared payment record count" eg:"15"`
	RefundCount     int    `json:"refundCount" dc:"Compared refund record count" eg:"5"`
	ResultUnknown   bool   `json:"resultUnknown" dc:"Whether the batch result is unknown" eg:"false"`
}
type ReconciliationBatchListRes struct {
	Batches []*ReconciliationBatchItem `json:"batches" dc:"Bounded daily reconciliation batch list" eg:"[]"`
	Total   int                        `json:"total" dc:"Exact number of visible matching records" eg:"123"`
}

type ReconciliationDifferenceListReq struct {
	g.Meta   `path:"/admin/payment-reconciliation/batches/{batchNo}/differences" method:"get" tags:"IDC Payment Reconciliation" summary:"List reconciliation differences" dc:"Return a bounded immutable difference list for one visible local batch." permission:"idc:admin:payment:reconciliation:view"`
	BatchNo  string `json:"batchNo" in:"path" v:"required|max-length:64" dc:"Exact reconciliation batch number" eg:"prb_123"`
	Page     int    `json:"page" d:"1" v:"between:1,1000000" dc:"One-based page number" eg:"1"`
	PageSize int    `json:"pageSize" d:"20" v:"between:1,100" dc:"Number of rows per page from 1 to 100" eg:"20"`
}
type ReconciliationDifferenceItem struct {
	LineNo                    string `json:"lineNo" dc:"Immutable difference line number" eg:"prl_123"`
	BatchNo                   string `json:"batchNo" dc:"Owning reconciliation batch number" eg:"prb_123"`
	RecordTypeCode            string `json:"recordTypeCode" dc:"Record type: payment or refund" eg:"payment"`
	AttemptNo                 string `json:"attemptNo" dc:"Local payment attempt number; empty for unmatched provider record" eg:"pay_123"`
	RefundAttemptNo           string `json:"refundAttemptNo" dc:"Local refund attempt number; empty for payment record" eg:"refund_123"`
	ExternalReference         string `json:"externalReference" dc:"Provider transaction reference" eg:"202608200001"`
	DifferenceCode            string `json:"differenceCode" dc:"Fixed difference category such as provider_success_local_not_posted" eg:"provider_success_local_not_posted"`
	CurrencyCode              string `json:"currencyCode" dc:"ISO 4217 currency code" eg:"CNY"`
	LocalStatusCode           string `json:"localStatusCode" dc:"Local immutable observed status" eg:"pending"`
	ChannelStatusCode         string `json:"channelStatusCode" dc:"Provider immutable observed status" eg:"succeeded"`
	ResolutionCode            string `json:"resolutionCode" dc:"Resolution status: pending, supplement_pending, applied, rejected" eg:"pending"`
	SupplementRequestNo       string `json:"supplementRequestNo" dc:"Controlled supplement request number; empty when not submitted" eg:"prs_123"`
	SupplementStatusCode      string `json:"supplementStatusCode" dc:"Supplement status: pending_review, applied, rejected; empty when not submitted" eg:"pending_review"`
	SupplementSubmitterUserID int    `json:"supplementSubmitterUserId" dc:"Submitting staff user ID; zero when not submitted" eg:"1001"`
	LocalAmountMinor          string `json:"localAmountMinor" dc:"Local amount in minor currency units as decimal string" eg:"10000"`
	ChannelAmountMinor        string `json:"channelAmountMinor" dc:"Provider amount in minor currency units as decimal string" eg:"10000"`
	Version                   int64  `json:"version" dc:"Difference optimistic-lock version" eg:"1"`
	SupplementVersion         int64  `json:"supplementVersion" dc:"Supplement optimistic-lock version; zero when not submitted" eg:"1"`
}
type ReconciliationDifferenceListRes struct {
	Differences []*ReconciliationDifferenceItem `json:"differences" dc:"Immutable reconciliation difference list" eg:"[]"`
	Total       int                             `json:"total" dc:"Exact number of visible matching records" eg:"123"`
}

type ReconciliationSupplementSubmitReq struct {
	g.Meta          `path:"/admin/payment-reconciliation/differences/{lineNo}/supplements" method:"post" tags:"IDC Payment Reconciliation" summary:"Submit a controlled payment supplement" dc:"Freeze an exactly matched provider-success difference for review. The caller cannot change payment, customer, channel, currency, amount, or provider transaction facts." permission:"idc:admin:payment:reconciliation:supplement-submit"`
	LineNo          string `json:"lineNo" in:"path" v:"required|max-length:64" dc:"Eligible immutable difference line number" eg:"prl_123"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Currently displayed difference version" eg:"1"`
}
type ReconciliationSupplementReviewReq struct {
	g.Meta              `path:"/admin/payment-reconciliation/supplements/{requestNo}/review" method:"post" tags:"IDC Payment Reconciliation" summary:"Review a controlled payment supplement" dc:"Require a different authenticated staff member. Approval rechecks every frozen fact and only queues the original idempotent payment-confirmation path; rejection accepts a fixed reason code." permission:"idc:admin:payment:reconciliation:supplement-review"`
	RequestNo           string `json:"requestNo" in:"path" v:"required|max-length:64" dc:"Controlled supplement request number" eg:"prs_123"`
	DecisionCode        string `json:"decisionCode" v:"required|in:approve,reject" dc:"Review decision: approve or reject" eg:"approve"`
	RejectionReasonCode string `json:"rejectionReasonCode" v:"max-length:48" dc:"Fixed rejection reason; required only for reject" eg:"evidence_insufficient"`
	ExpectedVersion     int64  `json:"expectedVersion" v:"required|min:1" dc:"Currently displayed supplement version" eg:"1"`
}
type ReconciliationSupplementItem struct {
	RequestNo           string `json:"requestNo" dc:"Controlled supplement request number" eg:"prs_123"`
	LineNo              string `json:"lineNo" dc:"Immutable difference line number" eg:"prl_123"`
	AttemptNo           string `json:"attemptNo" dc:"Exact local payment attempt number" eg:"pay_123"`
	ChannelNo           string `json:"channelNo" dc:"Exact local payment channel number" eg:"channel_alipay"`
	CurrencyCode        string `json:"currencyCode" dc:"Exact ISO 4217 currency code" eg:"CNY"`
	ExternalReference   string `json:"externalReference" dc:"Exact provider transaction reference" eg:"202608200001"`
	StatusCode          string `json:"statusCode" dc:"Supplement status: pending_review, applied, rejected" eg:"pending_review"`
	RejectionReasonCode string `json:"rejectionReasonCode" dc:"Fixed rejection reason; empty unless rejected" eg:"evidence_insufficient"`
	AmountMinor         string `json:"amountMinor" dc:"Exact amount in minor currency units as decimal string" eg:"10000"`
	SubmitterUserID     int    `json:"submitterUserId" dc:"Submitting staff user ID" eg:"1001"`
	ReviewerUserID      int    `json:"reviewerUserId" dc:"Reviewing staff user ID; zero before review" eg:"1002"`
	Version             int64  `json:"version" dc:"Supplement optimistic-lock version" eg:"1"`
}
type ReconciliationSupplementSubmitRes struct {
	Supplement *ReconciliationSupplementItem `json:"supplement" dc:"Created or reviewed controlled supplement" eg:"{}"`
}
type ReconciliationSupplementReviewRes = ReconciliationSupplementSubmitRes
