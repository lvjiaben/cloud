// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecycleTask is the golang structure of table plugin_idc_core_service_lifecycle_task for DAO operations like Where/Data.
type ServiceLifecycleTask struct {
	g.Meta                       `orm:"table:plugin_idc_core_service_lifecycle_task, do:true"`
	Id                           any        //
	TenantId                     any        //
	TaskNo                       any        //
	ServiceNo                    any        //
	CustomerNo                   any        //
	CurrencyCode                 any        //
	PolicyNo                     any        //
	PolicyVersion                any        //
	ActionCode                   any        //
	ReminderSequence             *int       //
	ExpectedServiceVersion       any        //
	ExpectedEntitlementExpiresAt *time.Time //
	DueAt                        *time.Time //
	StatusCode                   any        //
	BusinessOperationNo          *string    //
	PreviousRuntimeStatusCode    *string    //
	LastErrorCode                any        //
	IdempotencyDigest            any        //
	ContentDigest                any        //
	ClaimedAt                    *time.Time //
	DispatchedAt                 *time.Time //
	CompletedAt                  *time.Time //
	Version                      any        //
	CreatedAt                    *time.Time //
	UpdatedAt                    *time.Time //
	ResidualResources            any        // At most 16 bounded non-secret resource kind, opaque number, safe status and stable reason projections that still require reclamation
}
