package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingCouponBatchGenerateReq struct {
	g.Meta     `path:"/admin/marketing/coupon-codes/batches" method:"post" tags:"IDC Marketing Management" summary:"Generate one-time coupon codes" dc:"Generate one to one thousand case-insensitive coupon codes and return their only complete XLSX copy in this response." permission:"idc:admin:marketing:coupon-code:generate"`
	RequestNo  string `json:"requestNo" v:"required|max-length:128" dc:"Unique generation request number; a completed request cannot be downloaded again" eg:"coupon-batch-20260822-001"`
	CampaignNo string `json:"campaignNo" v:"required|max-length:40" dc:"Active coupon campaign business number" eg:"campaign-summer-2026"`
	BenefitNo  string `json:"benefitNo" v:"required|max-length:40" dc:"Active coupon benefit rule business number" eg:"benefit-summer-coupon"`
	CodeLength int    `json:"codeLength" v:"required|between:8,24" dc:"Length of every uppercase letter-and-number code" eg:"12"`
	Count      int    `json:"count" v:"required|between:1,1000" dc:"Number of one-time codes to generate" eg:"100"`
}

type MarketingCouponBatchGenerateRes struct{}
