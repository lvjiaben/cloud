// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ChannelSecretBinding is the golang structure of table plugin_idc_resource_channel_secret_binding for DAO operations like Where/Data.
type ChannelSecretBinding struct {
	g.Meta       `orm:"table:plugin_idc_resource_channel_secret_binding, do:true"`
	Id           any        //
	TenantId     any        //
	BindingNo    any        //
	ChannelNo    any        //
	BindingName  any        // Adapter-defined credential slot such as username or password
	SecretRef    any        // Sensitive routing metadata from secretcap Handle.OpaqueRef; never returned, logged, or audited
	StatusCode   any        // Recoverable owner lifecycle: preparing, retryable_failed, active, superseded, or revoked
	BoundAt      *time.Time //
	SupersededAt *time.Time //
	RevokedAt    *time.Time //
	Version      any        //
	CreatedBy    any        //
	UpdatedBy    any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
