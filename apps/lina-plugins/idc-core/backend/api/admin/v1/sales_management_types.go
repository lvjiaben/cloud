// This file defines bounded sales management response models and filters.
package v1

type SalesManagementLevel struct {
	PercentageBasisPoints *int   `json:"percentageBasisPoints" dc:"Default percentage charged in basis points; null means unset, 0 means free, 10000 means full price" eg:"8000"`
	PricingVersion        int64  `json:"pricingVersion" dc:"Independent default price revision; zero means never configured" eg:"1"`
	LevelNo               string `json:"levelNo" dc:"Sales management levelNo" eg:"example"`
	Name                  string `json:"name" dc:"Sales management name" eg:"example"`
	EnglishName           string `json:"englishName" dc:"Sales management englishName" eg:"example"`
	Status                string `json:"status" dc:"Sales management status" eg:"active"`
	Version               int64  `json:"version" dc:"Sales management version" eg:"1"`
}
type SalesManagementAgent struct {
	CustomerNo   string `json:"customerNo" dc:"Sales management customerNo" eg:"example"`
	CustomerCode string `json:"customerCode" dc:"Sales management customerCode" eg:"example"`
	CustomerName string `json:"customerName" dc:"Sales management customerName" eg:"example"`
	LevelNo      string `json:"levelNo" dc:"Sales management levelNo" eg:"example"`
	LevelName    string `json:"levelName" dc:"Sales management levelName" eg:"example"`
	Status       string `json:"status" dc:"Sales management status" eg:"active"`
	ExpiresAt    *int64 `json:"expiresAt" dc:"Optional Unix timestamp in milliseconds; omitted means no expiry" eg:"1"`
	Version      int64  `json:"version" dc:"Sales management version" eg:"1"`
}
type SalesManagementPrice struct {
	PriceNo             string `json:"priceNo" dc:"Sales management priceNo" eg:"example"`
	CustomerNo          string `json:"customerNo" dc:"Sales management customerNo" eg:"example"`
	CustomerCode        string `json:"customerCode" dc:"Sales management customerCode" eg:"example"`
	CustomerName        string `json:"customerName" dc:"Sales management customerName" eg:"example"`
	LevelNo             string `json:"levelNo" dc:"Sales management levelNo" eg:"example"`
	LevelName           string `json:"levelName" dc:"Sales management levelName" eg:"example"`
	ProductNo           string `json:"productNo" dc:"Sales management productNo" eg:"example"`
	ProductName         string `json:"productName" dc:"Sales management productName" eg:"example"`
	CurrencyCode        string `json:"currencyCode" dc:"Sales management currencyCode" eg:"example"`
	ChargeTypeCode      string `json:"chargeTypeCode" dc:"Sales management chargeTypeCode" eg:"example"`
	PeriodUnitCode      string `json:"periodUnitCode" dc:"Sales management periodUnitCode" eg:"example"`
	AdjustmentType      string `json:"adjustmentType" dc:"Sales management adjustmentType" eg:"example"`
	Status              string `json:"status" dc:"Sales management status" eg:"active"`
	FixedPriceMinor     string `json:"fixedPriceMinor" dc:"Sales management fixedPriceMinor" eg:"example"`
	PeriodCount         int    `json:"periodCount" dc:"Sales management periodCount" eg:"1"`
	DiscountBasisPoints *int   `json:"discountBasisPoints" dc:"Sales management discountBasisPoints" eg:"1"`
	Version             int64  `json:"version" dc:"Sales management version" eg:"1"`
}
type SalesManagementOwnership struct {
	CustomerNo         string `json:"customerNo" dc:"Sales management customerNo" eg:"example"`
	CustomerName       string `json:"customerName" dc:"Sales management customerName" eg:"example"`
	CustomerCode       string `json:"customerCode" dc:"Sales management customerCode" eg:"example"`
	ReferrerCustomerNo string `json:"referrerCustomerNo" dc:"Sales management referrerCustomerNo" eg:"example"`
	ReferrerName       string `json:"referrerName" dc:"Sales management referrerName" eg:"example"`
	ReferrerCode       string `json:"referrerCode" dc:"Sales management referrerCode" eg:"example"`
	StaffName          string `json:"staffName" dc:"Sales management staffName" eg:"example"`
	StaffUserID        int    `json:"staffUserId" dc:"Sales management staffUserId" eg:"1"`
	Version            int64  `json:"version" dc:"Sales management version" eg:"1"`
}
type SalesManagementRule struct {
	RuleNo           string `json:"ruleNo" dc:"Sales management ruleNo" eg:"example"`
	BeneficiaryType  string `json:"beneficiaryType" dc:"Sales management beneficiaryType" eg:"example"`
	OperationType    string `json:"operationType" dc:"Sales management operationType" eg:"example"`
	ProductNo        string `json:"productNo" dc:"Sales management productNo" eg:"example"`
	ProductName      string `json:"productName" dc:"Sales management productName" eg:"example"`
	CurrencyCode     string `json:"currencyCode" dc:"Sales management currencyCode" eg:"example"`
	CalculationType  string `json:"calculationType" dc:"Sales management calculationType" eg:"example"`
	FixedAmountMinor string `json:"fixedAmountMinor" dc:"Sales management fixedAmountMinor" eg:"example"`
	Status           string `json:"status" dc:"Sales management status" eg:"active"`
	RateBasisPoints  *int   `json:"rateBasisPoints" dc:"Sales management rateBasisPoints" eg:"1"`
	MaturityDays     int    `json:"maturityDays" dc:"Sales management maturityDays" eg:"1"`
	Version          int64  `json:"version" dc:"Sales management version" eg:"1"`
}
type SalesManagementPageQuery struct {
	Page     int    `json:"page" v:"min:1" dc:"Sales management page" eg:"1"`
	PageSize int    `json:"pageSize" v:"min:1|max:100" dc:"Sales management pageSize" eg:"1"`
	Search   string `json:"search" v:"max-length:100" dc:"Sales management search" eg:"example"`
}
