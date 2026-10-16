// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommissionAccrual is the golang structure of table plugin_idc_core_commission_accrual for DAO operations like Where/Data.
type CommissionAccrual struct {
	g.Meta                 `orm:"table:plugin_idc_core_commission_accrual, do:true"`
	Id                     any        //
	TenantId               any        //
	CommissionNo           any        //
	OrderNo                any        //
	InvoiceNo              any        //
	InvoiceItemNo          any        //
	BeneficiaryTypeCode    any        //
	BeneficiaryReference   any        //
	CurrencyCode           any        //
	RuleVersion            any        //
	BasisAmountMinor       any        //
	InvoiceItemAmountMinor any        //
	OriginalAmountMinor    any        //
	PendingAmountMinor     any        //
	MaturedAmountMinor     any        //
	ReversedAmountMinor    any        //
	StatusCode             any        //
	MaturesAt              *time.Time //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
	AgentQualificationNo   any        //
	AffiliateAttributionNo any        //
	SalesAssignmentNo      any        //
	CommissionRuleNo       any        //
	CommissionTierNo       any        //
	OperationTypeCode      any        //
}
