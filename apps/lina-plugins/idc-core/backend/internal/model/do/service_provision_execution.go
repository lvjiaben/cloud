// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceProvisionExecution is the golang structure of table plugin_idc_core_service_provision_execution for DAO operations like Where/Data.
type ServiceProvisionExecution struct {
	g.Meta                   `orm:"table:plugin_idc_core_service_provision_execution, do:true"`
	Id                       any        //
	TenantId                 any        //
	ExecutionNo              any        //
	OperationNo              any        //
	ServiceNo                any        //
	ResourceOwnerCode        any        //
	ProviderTaskNo           any        // Opaque resource-owner task reference; never a Provider task identifier
	ChannelNo                any        //
	StatusCode               any        //
	RequestDigest            any        //
	CapabilityEvidenceNo     any        //
	CapabilityEvidenceDigest any        //
	TaskEvidenceDigest       any        //
	LastFailureCode          any        //
	OutcomeReportStatus      any        //
	OutcomeReportedAt        *time.Time //
	ObservedAt               *time.Time //
	CompletedAt              *time.Time //
	Version                  any        //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
