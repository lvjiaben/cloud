// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerOnboardingIntent is the golang structure for table customer_onboarding_intent.
type CustomerOnboardingIntent struct {
	Id                     int64      `json:"id"                     orm:"id"                        description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                 description:""`
	IntentNo               string     `json:"intentNo"               orm:"intent_no"                 description:"Opaque onboarding intent business number"`
	TokenDigest            string     `json:"tokenDigest"            orm:"token_digest"              description:"SHA-256 digest of the single-use opaque token"`
	BrowserNonceDigest     string     `json:"browserNonceDigest"     orm:"browser_nonce_digest"      description:"SHA-256 digest of the browser-bound nonce"`
	NormalizedUsernameHash string     `json:"normalizedUsernameHash" orm:"normalized_username_hash"  description:"Digest of the normalized requested username"`
	ContactChannelCode     string     `json:"contactChannelCode"     orm:"contact_channel_code"      description:""`
	ContactHash            string     `json:"contactHash"            orm:"contact_hash"              description:"Digest of the verified contact destination"`
	CustomerTypeCode       string     `json:"customerTypeCode"       orm:"customer_type_code"        description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"               description:""`
	ReturnPath             string     `json:"returnPath"             orm:"return_path"               description:"Validated same-origin path used after onboarding"`
	HostRegistrationUserId int        `json:"hostRegistrationUserId" orm:"host_registration_user_id" description:"Host identity returned by native registration"`
	ConsumedByUserId       int        `json:"consumedByUserId"       orm:"consumed_by_user_id"       description:""`
	ResultCustomerNo       string     `json:"resultCustomerNo"       orm:"result_customer_no"        description:"Customer business number created by successful redemption"`
	VerifiedContactAt      *time.Time `json:"verifiedContactAt"      orm:"verified_contact_at"       description:""`
	ExpiresAt              *time.Time `json:"expiresAt"              orm:"expires_at"                description:"Intent expiry time"`
	ConsumedAt             *time.Time `json:"consumedAt"             orm:"consumed_at"               description:""`
	AttemptCount           int        `json:"attemptCount"           orm:"attempt_count"             description:""`
	Version                int64      `json:"version"                orm:"version"                   description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"                description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"                description:""`
	OperationDigest        string     `json:"operationDigest"        orm:"operation_digest"          description:""`
	RequestDigest          string     `json:"requestDigest"          orm:"request_digest"            description:""`
}
