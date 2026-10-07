// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionRule is the golang structure of table plugin_idc_core_sales_commission_rule for DAO operations like Where/Data.
type SalesCommissionRule struct {
	g.Meta              `orm:"table:plugin_idc_core_sales_commission_rule, do:true"`
	Id                  any        //
	TenantId            any        //
	RuleNo              any        //
	BeneficiaryTypeCode any        //
	OperationTypeCode   any        //
	ProductNo           *string    //
	CurrencyCode        any        //
	CalculationTypeCode any        //
	FixedAmountMinor    *int64     //
	RateBasisPoints     *int       //
	CapAmountMinor      *int64     //
	MaturityDays        any        //
	Priority            any        //
	StatusCode          any        //
	EffectiveAt         *time.Time //
	ExpiresAt           *time.Time //
	Version             any        //
	RuleDigest          any        //
	CreatedAt           *time.Time //
}
