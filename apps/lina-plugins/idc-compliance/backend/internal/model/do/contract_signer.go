// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractSigner is the golang structure of table plugin_idc_compliance_contract_signer for DAO operations like Where/Data.
type ContractSigner struct {
	g.Meta           `orm:"table:plugin_idc_compliance_contract_signer, do:true"`
	Id               any        //
	TenantId         any        //
	SignerNo         any        //
	FlowNo           any        //
	SequenceNo       any        //
	SubjectTypeCode  any        //
	SubjectNo        any        //
	RoleCode         any        //
	ProviderActorRef *string    //
	SnapshotSha256   any        //
	StatusCode       any        //
	SignedAt         *time.Time //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
