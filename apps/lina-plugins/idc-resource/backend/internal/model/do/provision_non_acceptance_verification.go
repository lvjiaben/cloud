// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProvisionNonAcceptanceVerification is the golang structure of table plugin_idc_resource_provision_non_acceptance_verification for DAO operations like Where/Data.
type ProvisionNonAcceptanceVerification struct {
	g.Meta             `orm:"table:plugin_idc_resource_provision_non_acceptance_verification, do:true"`
	TenantId           any        //
	TaskNo             any        //
	RequestDigest      any        //
	VerificationDigest any        //
	ActorUserId        any        //
	SafeNote           any        //
	VerifiedAt         *time.Time //
}
