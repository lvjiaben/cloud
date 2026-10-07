// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceRenewalPolicy is the golang structure of table plugin_idc_core_service_renewal_policy for DAO operations like Where/Data.
type ServiceRenewalPolicy struct {
	g.Meta                     `orm:"table:plugin_idc_core_service_renewal_policy, do:true"`
	Id                         any        //
	TenantId                   any        //
	PolicyNo                   any        //
	EntitlementNo              any        //
	PolicyVersion              any        //
	AutoRenewEnabled           any        //
	RenewalCycles              any        //
	LeadSeconds                any        //
	MaximumAttempts            any        //
	RetryIntervalSeconds       any        //
	FailureNotificationEnabled any        //
	ConfiguredBy               any        //
	ContentDigest              any        //
	CreatedAt                  *time.Time //
}
