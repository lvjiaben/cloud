// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerIdentityBinding is the golang structure of table plugin_idc_core_customer_identity_binding for DAO operations like Where/Data.
type CustomerIdentityBinding struct {
	g.Meta               `orm:"table:plugin_idc_core_customer_identity_binding, do:true"`
	Id                   any        //
	TenantId             any        //
	BindingNo            any        //
	SysUserId            any        // Host sys_user identity ID validated through host capabilities
	CustomerNo           any        // Bound customer business number
	SourceCode           any        // Binding source: onboarding, admin invitation, or migration
	SourceReferenceNo    any        // Opaque onboarding, invitation, or migration reference
	StatusCode           any        //
	ActivatedAt          *time.Time //
	RevokedAt            *time.Time //
	Version              any        //
	CreatedBy            any        //
	UpdatedBy            any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
	MustChangePassword   any        // Direct-created customer must replace the administrator-provided initial password before using account APIs
	IdempotencyKeyDigest any        // Irreversible digest of the administrator direct-create retry key
	RequestFingerprint   any        // Normalized direct-create request fingerprint excluding plaintext password
}
