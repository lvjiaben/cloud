// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialExportTicket is the golang structure of table plugin_idc_core_financial_export_ticket for DAO operations like Where/Data.
type FinancialExportTicket struct {
	g.Meta         `orm:"table:plugin_idc_core_financial_export_ticket, do:true"`
	Id             any        //
	TenantId       any        //
	TicketNo       any        //
	TokenDigest    any        //
	RequestNo      any        //
	RequestedBy    any        //
	PermissionCode any        //
	DataScopeCode  any        //
	ExpiresAt      *time.Time //
	ConsumedAt     *time.Time //
	CreatedAt      *time.Time //
}
