// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractAutoSignUsage is the golang structure of table plugin_idc_compliance_contract_auto_sign_usage for DAO operations like Where/Data.
type ContractAutoSignUsage struct {
	g.Meta                   `orm:"table:plugin_idc_compliance_contract_auto_sign_usage, do:true"`
	Id                       any        //
	TenantId                 any        //
	UsageNo                  any        //
	AuthorizationNo          any        //
	ContractNo               any        //
	CurrencyCode             any        //
	AmountMinor              any        //
	CommandIdempotencyDigest any        //
	CommandFingerprint       any        //
	ProviderAcceptedAt       *time.Time //
	CreatedAt                *time.Time //
}
