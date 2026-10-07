// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractTemplate is the golang structure for table contract_template.
type ContractTemplate struct {
	Id                        int64      `json:"id"                        orm:"id"                          description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                   description:""`
	TemplateNo                string     `json:"templateNo"                orm:"template_no"                 description:""`
	OriginCode                string     `json:"originCode"                orm:"origin_code"                 description:""`
	ProviderCode              *string    `json:"providerCode"              orm:"provider_code"               description:""`
	ProviderTemplateRef       *string    `json:"providerTemplateRef"       orm:"provider_template_ref"       description:"Opaque Tencent template identifier; it is never accepted from browser create or update commands"`
	ProviderStateCode         *string    `json:"providerStateCode"         orm:"provider_state_code"         description:""`
	ProviderLastSeenSyncNo    *string    `json:"providerLastSeenSyncNo"    orm:"provider_last_seen_sync_no"  description:""`
	ProviderUpdatedAt         *time.Time `json:"providerUpdatedAt"         orm:"provider_updated_at"         description:""`
	CreationIdempotencyDigest *string    `json:"creationIdempotencyDigest" orm:"creation_idempotency_digest" description:""`
	CreationFingerprint       *string    `json:"creationFingerprint"       orm:"creation_fingerprint"        description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                  description:""`
}
