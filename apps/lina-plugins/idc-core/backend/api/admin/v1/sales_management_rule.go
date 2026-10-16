// This file exposes real, permission-controlled commission-rules management.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementRuleListReq struct {
	g.Meta `path:"/admin/sales-management/commission-rules" method:"get" tags:"IDC Sales Management" summary:"List commission-rules" dc:"Return a bounded page of current commission-rules with display names." permission:"idc:admin:sales-channel:overview:view"`
	SalesManagementPageQuery
}
type SalesManagementRuleListRes struct {
	Items []SalesManagementRule `json:"items" dc:"Current page records" eg:"[]"`
	Total int                   `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementRuleCreateReq struct {
	g.Meta           `path:"/admin/sales-management/commission-rules" method:"post" tags:"IDC Sales Management" summary:"Create commission-rules" dc:"Persist one versioned commission-rules change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:commission:manage"`
	RuleNo           string `json:"ruleNo" v:"max-length:128" dc:"Sales management ruleNo" eg:"example"`
	BeneficiaryType  string `json:"beneficiaryType" v:"max-length:128" dc:"Sales management beneficiaryType" eg:"example"`
	OperationType    string `json:"operationType" v:"max-length:128" dc:"Sales management operationType" eg:"example"`
	ProductNo        string `json:"productNo" v:"max-length:128" dc:"Sales management productNo" eg:"example"`
	CurrencyCode     string `json:"currencyCode" v:"max-length:128" dc:"Sales management currencyCode" eg:"example"`
	CalculationType  string `json:"calculationType" v:"max-length:128" dc:"Sales management calculationType" eg:"example"`
	FixedAmountMinor string `json:"fixedAmountMinor" v:"max-length:128" dc:"Sales management fixedAmountMinor" eg:"example"`
	Status           string `json:"status" v:"required|in:active,inactive" dc:"Sales management status" eg:"active"`
	RateBasisPoints  *int   `json:"rateBasisPoints" dc:"Sales management rateBasisPoints" eg:"1"`
	MaturityDays     int    `json:"maturityDays" dc:"Sales management maturityDays" eg:"1"`
	ExpectedVersion  int64  `json:"expectedVersion" v:"in:0" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementRuleCreateRes struct {
	Item SalesManagementRule `json:"item" dc:"Saved record" eg:"{}"`
}
type SalesManagementRuleUpdateReq struct {
	g.Meta           `path:"/admin/sales-management/commission-rules" method:"put" tags:"IDC Sales Management" summary:"Update commission-rules" dc:"Persist one versioned commission-rules change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:commission:manage"`
	RuleNo           string `json:"ruleNo" v:"max-length:128" dc:"Sales management ruleNo" eg:"example"`
	BeneficiaryType  string `json:"beneficiaryType" v:"max-length:128" dc:"Sales management beneficiaryType" eg:"example"`
	OperationType    string `json:"operationType" v:"max-length:128" dc:"Sales management operationType" eg:"example"`
	ProductNo        string `json:"productNo" v:"max-length:128" dc:"Sales management productNo" eg:"example"`
	CurrencyCode     string `json:"currencyCode" v:"max-length:128" dc:"Sales management currencyCode" eg:"example"`
	CalculationType  string `json:"calculationType" v:"max-length:128" dc:"Sales management calculationType" eg:"example"`
	FixedAmountMinor string `json:"fixedAmountMinor" v:"max-length:128" dc:"Sales management fixedAmountMinor" eg:"example"`
	Status           string `json:"status" v:"required|in:active,inactive" dc:"Sales management status" eg:"active"`
	RateBasisPoints  *int   `json:"rateBasisPoints" dc:"Sales management rateBasisPoints" eg:"1"`
	MaturityDays     int    `json:"maturityDays" dc:"Sales management maturityDays" eg:"1"`
	ExpectedVersion  int64  `json:"expectedVersion" v:"required|min:1" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementRuleUpdateRes struct {
	Item SalesManagementRule `json:"item" dc:"Saved record" eg:"{}"`
}
