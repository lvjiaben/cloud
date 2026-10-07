// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// AutoSignAuth is the golang structure for table auto_sign_auth.
type AutoSignAuth struct {
	Id                          int64      `json:"id"                          orm:"id"                            description:""`
	TenantId                    int        `json:"tenantId"                    orm:"tenant_id"                     description:""`
	AuthorizationNo             string     `json:"authorizationNo"             orm:"authorization_no"              description:""`
	ProviderCode                string     `json:"providerCode"                orm:"provider_code"                 description:""`
	CurrencyCode                string     `json:"currencyCode"                orm:"currency_code"                 description:""`
	AllowRequiredOrder          bool       `json:"allowRequiredOrder"          orm:"allow_required_order"          description:""`
	AllowManualOptional         bool       `json:"allowManualOptional"         orm:"allow_manual_optional"         description:""`
	MaxAmountMinor              int64      `json:"maxAmountMinor"              orm:"max_amount_minor"              description:""`
	SuccessLimit                int64      `json:"successLimit"                orm:"success_limit"                 description:""`
	SuccessCount                int64      `json:"successCount"                orm:"success_count"                 description:""`
	StatusCode                  string     `json:"statusCode"                  orm:"status_code"                   description:""`
	ExpiresAt                   *time.Time `json:"expiresAt"                   orm:"expires_at"                    description:""`
	CreatedByUserId             int        `json:"createdByUserId"             orm:"created_by_user_id"            description:""`
	RevokedByUserId             *int       `json:"revokedByUserId"             orm:"revoked_by_user_id"            description:""`
	RevokedAt                   *time.Time `json:"revokedAt"                   orm:"revoked_at"                    description:""`
	CreationIdempotencyDigest   string     `json:"creationIdempotencyDigest"   orm:"creation_idempotency_digest"   description:""`
	CreationFingerprint         string     `json:"creationFingerprint"         orm:"creation_fingerprint"          description:""`
	RevocationIdempotencyDigest *string    `json:"revocationIdempotencyDigest" orm:"revocation_idempotency_digest" description:""`
	RevocationFingerprint       *string    `json:"revocationFingerprint"       orm:"revocation_fingerprint"        description:""`
	Version                     int64      `json:"version"                     orm:"version"                       description:""`
	CreatedAt                   *time.Time `json:"createdAt"                   orm:"created_at"                    description:""`
	UpdatedAt                   *time.Time `json:"updatedAt"                   orm:"updated_at"                    description:""`
}
