// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProvisionNonAcceptanceVerification is the golang structure for table provision_non_acceptance_verification.
type ProvisionNonAcceptanceVerification struct {
	TenantId           int64      `json:"tenantId"           orm:"tenant_id"           description:""`
	TaskNo             string     `json:"taskNo"             orm:"task_no"             description:""`
	RequestDigest      string     `json:"requestDigest"      orm:"request_digest"      description:""`
	VerificationDigest string     `json:"verificationDigest" orm:"verification_digest" description:""`
	ActorUserId        int64      `json:"actorUserId"        orm:"actor_user_id"       description:""`
	SafeNote           string     `json:"safeNote"           orm:"safe_note"           description:""`
	VerifiedAt         *time.Time `json:"verifiedAt"         orm:"verified_at"         description:""`
}
