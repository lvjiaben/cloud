// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Channel is the golang structure of table plugin_idc_notify_channel for DAO operations like Where/Data.
type Channel struct {
	g.Meta                     `orm:"table:plugin_idc_notify_channel, do:true"`
	TenantId                   any        //
	ChannelCode                any        //
	ChannelKind                any        //
	ProviderCode               any        //
	DisplayName                any        //
	StatusCode                 any        //
	BalanceState               any        //
	ConfigurationGeneration    any        //
	ConfigurationVersion       any        //
	NonSecretConfiguration     any        //
	CredentialSecretRef        any        //
	CredentialMaskedTail       any        //
	CredentialUpdatedAt        any        //
	CallbackVerifiedGeneration any        //
	CallbackVerifiedAt         any        //
	DisabledAt                 any        //
	UpdatedAt                  *time.Time //
	CreatedAt                  *time.Time //
}
