// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSavedFilter is the golang structure of table plugin_idc_core_financial_saved_filter for DAO operations like Where/Data.
type FinancialSavedFilter struct {
	g.Meta       `orm:"table:plugin_idc_core_financial_saved_filter, do:true"`
	Id           any        //
	TenantId     any        //
	FilterNo     any        //
	OwnerUserId  any        //
	ViewCode     any        //
	DisplayName  any        //
	FilterJson   any        //
	FilterDigest any        //
	Version      any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
