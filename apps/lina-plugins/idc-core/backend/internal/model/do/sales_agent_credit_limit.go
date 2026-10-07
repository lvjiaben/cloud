// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentCreditLimit is the golang structure of table plugin_idc_core_sales_agent_credit_limit for DAO operations like Where/Data.
type SalesAgentCreditLimit struct {
	g.Meta           `orm:"table:plugin_idc_core_sales_agent_credit_limit, do:true"`
	Id               any        //
	TenantId         any        //
	LimitNo          any        //
	QualificationNo  any        //
	CurrencyCode     any        //
	LimitAmountMinor any        //
	StatusCode       any        //
	EffectiveAt      *time.Time //
	ExpiresAt        *time.Time //
	CreatedAt        *time.Time //
}
