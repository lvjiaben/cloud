// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Channel is the golang structure for table channel.
type Channel struct {
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                    description:""`
	ChannelCode                string     `json:"channelCode"                orm:"channel_code"                 description:""`
	ChannelKind                string     `json:"channelKind"                orm:"channel_kind"                 description:""`
	ProviderCode               any        `json:"providerCode"               orm:"provider_code"                description:""`
	DisplayName                string     `json:"displayName"                orm:"display_name"                 description:""`
	StatusCode                 string     `json:"statusCode"                 orm:"status_code"                  description:""`
	BalanceState               string     `json:"balanceState"               orm:"balance_state"                description:""`
	ConfigurationGeneration    int64      `json:"configurationGeneration"    orm:"configuration_generation"     description:""`
	ConfigurationVersion       int64      `json:"configurationVersion"       orm:"configuration_version"        description:""`
	NonSecretConfiguration     string     `json:"nonSecretConfiguration"     orm:"non_secret_configuration"     description:""`
	CredentialSecretRef        any        `json:"credentialSecretRef"        orm:"credential_secret_ref"        description:""`
	CredentialMaskedTail       string     `json:"credentialMaskedTail"       orm:"credential_masked_tail"       description:""`
	CredentialUpdatedAt        any        `json:"credentialUpdatedAt"        orm:"credential_updated_at"        description:""`
	CallbackVerifiedGeneration int64      `json:"callbackVerifiedGeneration" orm:"callback_verified_generation" description:""`
	CallbackVerifiedAt         any        `json:"callbackVerifiedAt"         orm:"callback_verified_at"         description:""`
	DisabledAt                 any        `json:"disabledAt"                 orm:"disabled_at"                  description:""`
	UpdatedAt                  *time.Time `json:"updatedAt"                  orm:"updated_at"                   description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                   description:""`
}
