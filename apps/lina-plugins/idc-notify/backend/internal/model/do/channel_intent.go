// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ChannelIntent is the golang structure of table plugin_idc_notify_channel_intent for DAO operations like Where/Data.
type ChannelIntent struct {
	g.Meta                        `orm:"table:plugin_idc_notify_channel_intent, do:true"`
	TenantId                      any        //
	OperationNo                   any        //
	ChannelCode                   any        //
	OperationCode                 any        //
	StatusCode                    any        //
	ActorUserId                   any        //
	BaseStatusCode                any        //
	BaseVersion                   any        //
	RequestShapeDigest            any        //
	DesiredProviderCode           any        //
	DesiredNonSecretConfiguration any        //
	SecretOperationKey            any        //
	CredentialSecretRef           any        //
	SecretExpectedUpdatedAt       any        //
	SecretResultUpdatedAt         any        //
	SecretResultMaskedTail        any        //
	AvailableAt                   any        //
	LeaseToken                    any        //
	LeaseExpiresAt                any        //
	AttemptCount                  any        //
	LastErrorCode                 any        //
	CompletedAt                   any        //
	CreatedAt                     *time.Time //
	UpdatedAt                     *time.Time //
}
