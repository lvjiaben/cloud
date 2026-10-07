// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialExportRequest is the golang structure of table plugin_idc_core_financial_export_request for DAO operations like Where/Data.
type FinancialExportRequest struct {
	g.Meta                 `orm:"table:plugin_idc_core_financial_export_request, do:true"`
	Id                     any        //
	TenantId               any        //
	RequestNo              any        //
	ExportTypeCode         any        //
	FilterJson             any        //
	FilterDigest           any        //
	PermissionCode         any        //
	DataScopeCode          any        //
	RequestedBy            any        //
	IdempotencyDigest      any        //
	AuthorizationCheckedAt *time.Time //
	StatusCode             any        //
	FailureCode            any        //
	GeneratedAt            *time.Time //
	CreatedAt              *time.Time //
}
