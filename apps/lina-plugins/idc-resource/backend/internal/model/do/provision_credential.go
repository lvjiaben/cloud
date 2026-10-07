// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProvisionCredential is the golang structure of table plugin_idc_resource_provision_credential for DAO operations like Where/Data.
type ProvisionCredential struct {
	g.Meta       `orm:"table:plugin_idc_resource_provision_credential, do:true"`
	Id           any        //
	TenantId     any        //
	CredentialNo any        //
	ServiceNo    any        //
	TaskNo       any        //
	ChannelNo    any        //
	SecretRef    any        // Sensitive secretcap handle reference; never returned, logged, audited, or sent as ordinary JSON
	StatusCode   any        //
	RevealedAt   *time.Time //
	RevokedAt    *time.Time //
	Version      any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
