// This file exposes separate commission balances and offline payout evidence.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementCommissionAccount struct {
	AccountNo            string `json:"accountNo" dc:"Stable commission account number, isolated from customer spending balance" eg:"SCA_demo"`
	BeneficiaryType      string `json:"beneficiaryType" dc:"Beneficiary type: sales for an employee, affiliate for a referring customer" eg:"sales"`
	BeneficiaryReference string `json:"beneficiaryReference" dc:"Employee user ID for sales, or customer number for affiliate" eg:"81"`
	BeneficiaryName      string `json:"beneficiaryName" dc:"Visible beneficiary display name" eg:"Alice"`
	CurrencyCode         string `json:"currencyCode" dc:"Account currency; amounts never combine different currencies" eg:"CNY"`
	CreditedMinor        string `json:"creditedMinor" dc:"Cumulative earned commission as an integer string in currency minor units" eg:"1000"`
	PendingMinor         string `json:"pendingMinor" dc:"Commission waiting to mature as an integer string in currency minor units" eg:"200"`
	AvailableMinor       string `json:"availableMinor" dc:"Mature commission available for offline payout, in currency minor units" eg:"800"`
	PaidMinor            string `json:"paidMinor" dc:"Commission already paid offline, in currency minor units" eg:"0"`
	ReversedMinor        string `json:"reversedMinor" dc:"Commission reversed because of refunds, in currency minor units" eg:"0"`
	RecoveryDueMinor     string `json:"recoveryDueMinor" dc:"Already-paid commission still owed after refunds; positive value blocks payout" eg:"0"`
	Version              int64  `json:"version" dc:"Current account revision used to prevent conflicting writes" eg:"1"`
}
type SalesManagementCommissionAccountListReq struct {
	g.Meta `path:"/admin/sales-management/commission/accounts" method:"get" tags:"IDC Sales Management" summary:"List commission accounts" dc:"Return a scoped page of commission accounts; money remains isolated from customer spending balances." permission:"idc:admin:sales-channel:commission:view"`
	SalesManagementPageQuery
}
type SalesManagementCommissionAccountListRes struct {
	Items []SalesManagementCommissionAccount `json:"items" dc:"Current page records" eg:"[]"`
	Total int                                `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementCommissionRecord struct {
	InvoiceNo            string `json:"invoiceNo" dc:"Source invoice number, including renewal invoices" eg:"INV_example"`
	CommissionNo         string `json:"commissionNo" dc:"Immutable commission accrual number" eg:"COM_demo"`
	OrderNo              string `json:"orderNo" dc:"Source purchase order number; empty for renewal-only invoice" eg:"ORD_demo"`
	CustomerName         string `json:"customerName" dc:"Source paying customer display name" eg:"Alice"`
	BeneficiaryType      string `json:"beneficiaryType" dc:"Beneficiary type: sales for an employee, affiliate for a referring customer" eg:"sales"`
	BeneficiaryReference string `json:"beneficiaryReference" dc:"Employee user ID for sales, or customer number for affiliate" eg:"81"`
	BeneficiaryName      string `json:"beneficiaryName" dc:"Visible beneficiary display name" eg:"Alice"`
	CurrencyCode         string `json:"currencyCode" dc:"Account currency; amounts never combine different currencies" eg:"CNY"`
	Status               string `json:"status" dc:"Accrual status: pending_maturity waiting to mature, matured, partially_reversed, or reversed" eg:"pending_maturity"`
	BasisMinor           string `json:"basisMinor" dc:"Actual received amount used to calculate this commission, in currency minor units" eg:"10000"`
	OriginalMinor        string `json:"originalMinor" dc:"Commission at accrual before refund reversals, in currency minor units" eg:"1000"`
	PendingMinor         string `json:"pendingMinor" dc:"Commission waiting to mature as an integer string in currency minor units" eg:"200"`
	MaturedMinor         string `json:"maturedMinor" dc:"Commission that has matured, in currency minor units" eg:"800"`
	ReversedMinor        string `json:"reversedMinor" dc:"Commission reversed because of refunds, in currency minor units" eg:"0"`
	MaturesAt            *int64 `json:"maturesAt" dc:"Unix timestamp in milliseconds" eg:"1791590400000"`
}
type SalesManagementCommissionRecordListReq struct {
	g.Meta `path:"/admin/sales-management/commission/records" method:"get" tags:"IDC Sales Management" summary:"List commission records" dc:"Return a scoped page of commission records; money remains isolated from customer spending balances." permission:"idc:admin:sales-channel:commission:view"`
	SalesManagementPageQuery
}
type SalesManagementCommissionRecordListRes struct {
	Items []SalesManagementCommissionRecord `json:"items" dc:"Current page records" eg:"[]"`
	Total int                               `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementCommissionPayout struct {
	PayoutNo          string `json:"payoutNo" dc:"Immutable offline payout evidence number" eg:"SPO_demo"`
	AccountNo         string `json:"accountNo" dc:"Stable commission account number, isolated from customer spending balance" eg:"SCA_demo"`
	BeneficiaryName   string `json:"beneficiaryName" dc:"Visible beneficiary display name" eg:"Alice"`
	CurrencyCode      string `json:"currencyCode" dc:"Account currency; amounts never combine different currencies" eg:"CNY"`
	AmountMinor       string `json:"amountMinor" dc:"Offline payout amount as a positive integer string in currency minor units" eg:"800"`
	EvidenceReference string `json:"evidenceReference" dc:"Bank transfer reference or payment evidence description, 6 to 512 characters" eg:"Bank transfer 20261010-001"`
	PaidAt            *int64 `json:"paidAt" dc:"Unix timestamp in milliseconds" eg:"1791590400000"`
	CreatedAt         *int64 `json:"createdAt" dc:"Unix timestamp in milliseconds" eg:"1791590400000"`
	RecordedBy        int    `json:"recordedBy" dc:"Employee user ID that recorded this offline payment" eg:"81"`
}
type SalesManagementCommissionPayoutListReq struct {
	g.Meta `path:"/admin/sales-management/commission/payouts" method:"get" tags:"IDC Sales Management" summary:"List commission payouts" dc:"Return a scoped page of commission payouts; money remains isolated from customer spending balances." permission:"idc:admin:sales-channel:commission:view"`
	SalesManagementPageQuery
}
type SalesManagementCommissionPayoutListRes struct {
	Items []SalesManagementCommissionPayout `json:"items" dc:"Current page records" eg:"[]"`
	Total int                               `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementCommissionPayoutCreateReq struct {
	g.Meta            `path:"/admin/sales-management/commission/payouts" method:"post" tags:"IDC Sales Management" summary:"Record offline commission payout" dc:"Record a completed offline payment with evidence and optimistic account revision; no external transfer is sent." permission:"idc:admin:sales-channel:commission:payout"`
	AccountNo         string `json:"accountNo" v:"required|max-length:40" dc:"Stable commission account number, isolated from customer spending balance" eg:"SCA_demo"`
	AmountMinor       string `json:"amountMinor" v:"required|max-length:16" dc:"Offline payout amount as a positive integer string in currency minor units" eg:"800"`
	EvidenceReference string `json:"evidenceReference" v:"required|min-length:6|max-length:512" dc:"Bank transfer reference or payment evidence description, 6 to 512 characters" eg:"Bank transfer 20261010-001"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|max-length:128" dc:"Unique payout request key; retries must preserve the key and payload" eg:"sales-payout-20261010-001"`
	PaidAt            int64  `json:"paidAt" v:"required|min:1" dc:"Unix timestamp in milliseconds" eg:"1791590400000"`
	ExpectedVersion   int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected commission account revision; stale revisions are rejected" eg:"1"`
}
type SalesManagementCommissionPayoutCreateRes struct {
	Item SalesManagementCommissionPayout `json:"item" dc:"Recorded offline payout" eg:"{}"`
}
