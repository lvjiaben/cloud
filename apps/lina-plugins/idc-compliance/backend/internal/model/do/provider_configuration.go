// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProviderConfiguration is the golang structure of table plugin_idc_compliance_provider_configuration for DAO operations like Where/Data.
type ProviderConfiguration struct {
	g.Meta                  `orm:"table:plugin_idc_compliance_provider_configuration, do:true"`
	Id                      any        //
	TenantId                any        //
	ConfigurationNo         any        //
	ProviderCode            any        //
	StatusCode              any        //
	SceneId                 any        // Merchant-owned Aliyun financial-grade identity verification scene ID; endpoint, service, version, FACE code, and identity type are fixed in code
	CredentialSecretRef     *string    // Sensitive secretcap routing reference for one atomic RAM AccessKey credential envelope; plaintext and ciphertext are forbidden
	CredentialGeneration    any        //
	LastMutationDigest      *string    // SHA-256 digest of the bounded operator idempotency key; credentials are excluded
	MutationOperationKey    *string    // Internal random secretcap idempotency key retained only for audited recovery; never exposed by API, logs, or audit payloads
	MutationSecretUpdatedAt *time.Time // Safe secretcap optimistic fence retained to resume or diagnose the last idempotent credential replacement
	LastValidationCode      any        //
	ValidatedAt             *time.Time //
	Version                 any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
