// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProviderConfiguration is the golang structure for table provider_configuration.
type ProviderConfiguration struct {
	Id                      int64      `json:"id"                      orm:"id"                         description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                  description:""`
	ConfigurationNo         string     `json:"configurationNo"         orm:"configuration_no"           description:""`
	ProviderCode            string     `json:"providerCode"            orm:"provider_code"              description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"                description:""`
	SceneId                 string     `json:"sceneId"                 orm:"scene_id"                   description:"Merchant-owned Aliyun financial-grade identity verification scene ID; endpoint, service, version, FACE code, and identity type are fixed in code"`
	CredentialSecretRef     *string    `json:"credentialSecretRef"     orm:"credential_secret_ref"      description:"Sensitive secretcap routing reference for one atomic RAM AccessKey credential envelope; plaintext and ciphertext are forbidden"`
	CredentialGeneration    int64      `json:"credentialGeneration"    orm:"credential_generation"      description:""`
	LastMutationDigest      *string    `json:"lastMutationDigest"      orm:"last_mutation_digest"       description:"SHA-256 digest of the bounded operator idempotency key; credentials are excluded"`
	MutationOperationKey    *string    `json:"mutationOperationKey"    orm:"mutation_operation_key"     description:"Internal random secretcap idempotency key retained only for audited recovery; never exposed by API, logs, or audit payloads"`
	MutationSecretUpdatedAt *time.Time `json:"mutationSecretUpdatedAt" orm:"mutation_secret_updated_at" description:"Safe secretcap optimistic fence retained to resume or diagnose the last idempotent credential replacement"`
	LastValidationCode      string     `json:"lastValidationCode"      orm:"last_validation_code"       description:""`
	ValidatedAt             *time.Time `json:"validatedAt"             orm:"validated_at"               description:""`
	Version                 int64      `json:"version"                 orm:"version"                    description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                 description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                 description:""`
}
