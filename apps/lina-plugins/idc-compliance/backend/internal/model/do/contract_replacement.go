// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractReplacement is the golang structure of table plugin_idc_compliance_contract_replacement for DAO operations like Where/Data.
type ContractReplacement struct {
	g.Meta            `orm:"table:plugin_idc_compliance_contract_replacement, do:true"`
	Id                any        //
	TenantId          any        //
	ReplacementNo     any        //
	OldContractNo     any        //
	NewContractNo     any        //
	IdempotencySha256 any        //
	CreatedAt         *time.Time //
}
