// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSnapshot is the golang structure of table plugin_idc_resource_financial_snapshot for DAO operations like Where/Data.
type FinancialSnapshot struct {
	g.Meta              `orm:"table:plugin_idc_resource_financial_snapshot, do:true"`
	Id                  any        //
	TenantId            any        //
	SnapshotNo          any        //
	ChannelNo           any        //
	SnapshotKind        any        //
	ExternalResourceRef any        //
	CurrencyCode        any        //
	Amount              any        // Non-negative Provider amount in the exact ISO currency; never converted implicitly
	ExpiresAt           *time.Time //
	Projection          any        //
	ObservedAt          *time.Time //
	CreatedAt           *time.Time //
}
