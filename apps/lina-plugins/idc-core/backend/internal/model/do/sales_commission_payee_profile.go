// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayeeProfile is the golang structure of table plugin_idc_core_sales_commission_payee_profile for DAO operations like Where/Data.
type SalesCommissionPayeeProfile struct {
	g.Meta                   `orm:"table:plugin_idc_core_sales_commission_payee_profile, do:true"`
	Id                       any        //
	TenantId                 any        //
	ProfileNo                any        //
	BeneficiaryTypeCode      any        //
	BeneficiaryReference     any        //
	PayeeTypeCode            any        //
	ProtectedObjectReference any        //
	AccountDigest            any        //
	StatusCode               any        //
	Version                  any        //
	VerifiedAt               *time.Time //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
