// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractSignerProfile is the golang structure for table contract_signer_profile.
type ContractSignerProfile struct {
	Id                          int64      `json:"id"                          orm:"id"                            description:""`
	TenantId                    int        `json:"tenantId"                    orm:"tenant_id"                     description:""`
	SignerNo                    string     `json:"signerNo"                    orm:"signer_no"                     description:""`
	CustomerNo                  string     `json:"customerNo"                  orm:"customer_no"                   description:""`
	ProfileSnapshotRef          string     `json:"profileSnapshotRef"          orm:"profile_snapshot_ref"          description:""`
	ProfileSnapshotSha256       string     `json:"profileSnapshotSha256"       orm:"profile_snapshot_sha256"       description:""`
	MaskedName                  string     `json:"maskedName"                  orm:"masked_name"                   description:""`
	MaskedIdentityNo            string     `json:"maskedIdentityNo"            orm:"masked_identity_no"            description:""`
	MaskedPhone                 string     `json:"maskedPhone"                 orm:"masked_phone"                  description:""`
	ConsentPolicyVersion        string     `json:"consentPolicyVersion"        orm:"consent_policy_version"        description:""`
	ConsentOccurredAt           *time.Time `json:"consentOccurredAt"           orm:"consent_occurred_at"           description:""`
	IdentityStatusCode          string     `json:"identityStatusCode"          orm:"identity_status_code"          description:""`
	IdentityAttemptNo           *string    `json:"identityAttemptNo"           orm:"identity_attempt_no"           description:""`
	IdentityVerificationVersion int64      `json:"identityVerificationVersion" orm:"identity_verification_version" description:""`
	IdentityVerifiedAt          *time.Time `json:"identityVerifiedAt"          orm:"identity_verified_at"          description:""`
	PhoneStatusCode             string     `json:"phoneStatusCode"             orm:"phone_status_code"             description:""`
	PhoneVerificationNo         *string    `json:"phoneVerificationNo"         orm:"phone_verification_no"         description:""`
	PhoneVerificationVersion    int64      `json:"phoneVerificationVersion"    orm:"phone_verification_version"    description:""`
	PhoneVerifiedAt             *time.Time `json:"phoneVerifiedAt"             orm:"phone_verified_at"             description:""`
	StatusCode                  string     `json:"statusCode"                  orm:"status_code"                   description:""`
	AuthorizationVersion        int64      `json:"authorizationVersion"        orm:"authorization_version"         description:""`
	CreationDigest              string     `json:"creationDigest"              orm:"creation_digest"               description:""`
	DisabledAt                  *time.Time `json:"disabledAt"                  orm:"disabled_at"                   description:""`
	Version                     int64      `json:"version"                     orm:"version"                       description:""`
	CreatedAt                   *time.Time `json:"createdAt"                   orm:"created_at"                    description:""`
	UpdatedAt                   *time.Time `json:"updatedAt"                   orm:"updated_at"                    description:""`
}
