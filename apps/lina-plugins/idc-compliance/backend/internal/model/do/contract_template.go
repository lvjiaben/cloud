// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplate is the golang structure of table plugin_idc_compliance_contract_template for DAO operations like Where/Data.
type ContractTemplate struct {
	g.Meta                    `orm:"table:plugin_idc_compliance_contract_template, do:true"`
	Id                        any        //
	TenantId                  any        //
	TemplateNo                any        //
	OriginCode                any        //
	ProviderCode              *string    //
	ProviderTemplateRef       *string    // Opaque Tencent template identifier; it is never accepted from browser create or update commands
	ProviderStateCode         *string    //
	ProviderLastSeenSyncNo    *string    //
	ProviderUpdatedAt         *time.Time //
	CreationIdempotencyDigest *string    //
	CreationFingerprint       *string    //
	CreatedAt                 *time.Time //
}
