// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerOperationExecution is the golang structure of table plugin_idc_core_server_operation_execution for DAO operations like Where/Data.
type ServerOperationExecution struct {
	g.Meta                    `orm:"table:plugin_idc_core_server_operation_execution, do:true"`
	Id                        any        //
	TenantId                  any        //
	ExecutionNo               any        //
	OperationNo               any        //
	ServiceNo                 any        //
	ActionCode                any        //
	ResourceOwnerCode         any        //
	ChannelNo                 any        //
	PoolNo                    any        //
	ExpectedServiceVersion    any        //
	OriginalRuntimeStatusCode any        //
	Parameters                any        //
	SecretBindings            any        // Opaque input secret references only; plaintext is forbidden
	StatusCode                any        //
	ProviderTaskNo            *string    //
	RequestDigest             any        //
	CapabilityEvidenceNo      *string    //
	CapabilityEvidenceDigest  *string    //
	TaskEvidenceDigest        *string    //
	LastFailureCode           any        //
	OutputBindings            any        // Opaque confidential result references with one-time or short-lived delivery policy
	ObservedAt                *time.Time //
	CompletedAt               *time.Time //
	Version                   any        //
	CreatedAt                 *time.Time //
	UpdatedAt                 *time.Time //
}
