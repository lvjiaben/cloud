// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionSettlementItem is the golang structure for table sales_commission_settlement_item.
type SalesCommissionSettlementItem struct {
	Id                   int64      `json:"id"                   orm:"id"                    description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"             description:""`
	ItemNo               string     `json:"itemNo"               orm:"item_no"               description:""`
	SettlementNo         string     `json:"settlementNo"         orm:"settlement_no"         description:""`
	CommissionNo         string     `json:"commissionNo"         orm:"commission_no"         description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"         description:""`
	BeneficiaryTypeCode  string     `json:"beneficiaryTypeCode"  orm:"beneficiary_type_code" description:""`
	BeneficiaryReference string     `json:"beneficiaryReference" orm:"beneficiary_reference" description:""`
	AmountMinor          int64      `json:"amountMinor"          orm:"amount_minor"          description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"            description:""`
}
