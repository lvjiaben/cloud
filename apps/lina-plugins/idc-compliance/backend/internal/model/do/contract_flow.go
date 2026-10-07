// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractFlow is the golang structure of table plugin_idc_compliance_contract_flow for DAO operations like Where/Data.
type ContractFlow struct {
	g.Meta                `orm:"table:plugin_idc_compliance_contract_flow, do:true"`
	Id                    any        //
	TenantId              any        //
	FlowNo                any        //
	ContractNo            any        //
	SequenceNo            any        //
	PurposeCode           any        //
	ProviderCode          any        //
	ExternalFlowRef       *string    //
	IdempotencySha256     any        //
	RequestSnapshotRef    any        //
	RequestSnapshotSha256 any        //
	StatusCode            any        //
	AvailableAt           *time.Time //
	NextQueryAt           *time.Time //
	ExpiresAt             *time.Time //
	SubmittedAt           *time.Time //
	CompletedAt           *time.Time //
	FailureCode           any        //
	Version               any        //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
	GatewayRouteNo        *string    // Opaque installation-local supplier-write route; never a Tencent flow identifier
	ProviderStatus        *string    //
	GatewayRouteVersion   any        //
}
