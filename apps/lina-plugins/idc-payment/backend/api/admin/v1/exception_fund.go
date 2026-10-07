// This file declares pending exception-fund pages and existing disposition
// actions. Pagination does not allocate or refund funds.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ExceptionFundListReq struct {
	g.Meta   `path:"/admin/payment-exception-funds" method:"get" tags:"IDC Payment Exception Fund" summary:"List pending payment exception funds" dc:"Return one bounded pending or disputed fund page with its exact matching total." permission:"idc:admin:payment:exception-fund:view"`
	Page     int `json:"page" d:"1" v:"between:1,1000000" dc:"One-based page number" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"between:1,100" dc:"Number of rows per page from 1 to 100" eg:"20"`
}
type ExceptionFundItem struct {
	ExceptionNo    string `json:"exceptionNo" dc:"Exception fund number" eg:"pex_123"`
	AttemptNo      string `json:"attemptNo" dc:"Original payment attempt number" eg:"pay_123"`
	OrderNo        string `json:"orderNo" dc:"Cancelled original order number" eg:"ORD_123"`
	ChannelNo      string `json:"channelNo" dc:"Original payment channel number" eg:"channel_alipay"`
	ProviderCode   string `json:"providerCode" dc:"Original provider code" eg:"alipay"`
	CurrencyCode   string `json:"currencyCode" dc:"ISO 4217 currency code" eg:"CNY"`
	AmountMinor    string `json:"amountMinor" dc:"Amount in minor currency units" eg:"10000"`
	ReasonCode     string `json:"reasonCode" dc:"Exception reason" eg:"late_success"`
	StatusCode     string `json:"statusCode" dc:"Fund status" eg:"pending"`
	CreatedAt      int64  `json:"createdAt" dc:"Creation time as Unix milliseconds" eg:"1787212800000"`
	ActionNo       string `json:"actionNo" dc:"Current disposition action number" eg:"pfa_123"`
	ActionCode     string `json:"actionCode" dc:"Current disposition action" eg:"allocate"`
	ActionStatus   string `json:"actionStatus" dc:"Current action status" eg:"manual_pending"`
	TargetOrderNo  string `json:"targetOrderNo" dc:"Chosen new order number" eg:"ORD_456"`
	ProofReference string `json:"proofReference" dc:"Governed host file reference for the manual refund proof" eg:"host-file:123"`
	Version        int64  `json:"version" dc:"Current action version" eg:"2"`
}
type ExceptionFundListRes struct {
	Funds []*ExceptionFundItem `json:"funds" dc:"Bounded exception fund list" eg:"[]"`
	Total int                  `json:"total" dc:"Exact number of visible matching records" eg:"123"`
}

type ExceptionFundAllocateReq struct {
	g.Meta        `path:"/admin/payment-exception-funds/{exceptionNo}/allocation" method:"post" tags:"IDC Payment Exception Fund" summary:"Allocate a late payment to a new order" permission:"idc:admin:payment:exception-fund:allocate"`
	ExceptionNo   string `json:"exceptionNo" in:"path" v:"required|max-length:64" dc:"Exception fund number" eg:"pex_123"`
	TargetOrderNo string `json:"targetOrderNo" v:"required|max-length:64" dc:"Same-customer unpaid new order number" eg:"ORD_456"`
}
type ExceptionFundAllocateRes struct {
	Fund *ExceptionFundItem `json:"fund" dc:"Updated exception fund" eg:"{}"`
}

type ExceptionFundRefundReq struct {
	g.Meta      `path:"/admin/payment-exception-funds/{exceptionNo}/original-refund" method:"post" tags:"IDC Payment Exception Fund" summary:"Request an original-source refund" permission:"idc:admin:payment:exception-fund:refund"`
	ExceptionNo string `json:"exceptionNo" in:"path" v:"required|max-length:64" dc:"Exception fund number" eg:"pex_123"`
}
type ExceptionFundRefundRes = ExceptionFundAllocateRes

type ExceptionFundManualRefundConfirmReq struct {
	g.Meta          `path:"/admin/payment-exception-funds/{exceptionNo}/manual-refund-confirmation" method:"post" tags:"IDC Payment Exception Fund" summary:"Confirm a completed manual original-source refund" permission:"idc:admin:payment:exception-fund:refund-confirm"`
	ExceptionNo     string `json:"exceptionNo" in:"path" v:"required|max-length:64" dc:"Exception fund number" eg:"pex_123"`
	ProofReference  string `json:"proofReference" v:"required|max-length:256" dc:"Governed host file reference for the uploaded refund proof" eg:"host-file:123"`
	ProofDigest     string `json:"proofDigest" v:"required|regex:^[0-9a-f]{64}$" dc:"SHA-256 digest of the retained refund proof" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Currently displayed action version" eg:"2"`
}
type ExceptionFundManualRefundConfirmRes = ExceptionFundAllocateRes

type ExceptionFundProofUploadReq struct {
	g.Meta      `path:"/admin/payment-exception-funds/{exceptionNo}/manual-refund-proof" method:"post" mime:"multipart/form-data" tags:"IDC Payment Exception Fund" summary:"Upload a manual original-refund proof" permission:"idc:admin:payment:exception-fund:refund-confirm"`
	ExceptionNo string `json:"exceptionNo" in:"path" v:"required|max-length:64" dc:"Exception fund number" eg:"pex_123"`
}
type ExceptionFundProofUploadRes struct {
	ProofReference string `json:"proofReference" dc:"Governed host file reference" eg:"host-file:123"`
	ProofDigest    string `json:"proofDigest" dc:"SHA-256 digest of the uploaded proof" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	FileName       string `json:"fileName" dc:"Uploaded proof file name" eg:"refund-proof.pdf"`
	SizeBytes      string `json:"sizeBytes" dc:"Uploaded proof size in bytes" eg:"102400"`
}
