// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionMaturity is the golang structure of table plugin_idc_core_sales_commission_maturity for DAO operations like Where/Data.
type SalesCommissionMaturity struct {
	g.Meta               `orm:"table:plugin_idc_core_sales_commission_maturity, do:true"`
	Id                   any        //
	TenantId             any        //
	MaturityNo           any        //
	CommissionNo         any        //
	AttemptNo            any        //
	AccountNo            any        //
	CurrencyCode         any        //
	BeneficiaryTypeCode  any        //
	BeneficiaryReference any        //
	AmountMinor          any        //
	StatusCode           any        //
	ReasonCode           any        //
	EvidenceDigest       any        //
	ProcessedAt          *time.Time //
}
