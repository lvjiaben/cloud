// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FulfillmentNonAcceptanceVerification is the golang structure for table fulfillment_non_acceptance_verification.
type FulfillmentNonAcceptanceVerification struct {
	TenantId            int64      `json:"tenantId"            orm:"tenant_id"             description:""`
	ProviderTaskNo      string     `json:"providerTaskNo"      orm:"provider_task_no"      description:""`
	IntentNo            string     `json:"intentNo"            orm:"intent_no"             description:""`
	TaskNo              string     `json:"taskNo"              orm:"task_no"               description:""`
	PreviousTaskVersion int64      `json:"previousTaskVersion" orm:"previous_task_version" description:""`
	ResultingVersion    int64      `json:"resultingVersion"    orm:"resulting_version"     description:""`
	IdempotencyKey      string     `json:"idempotencyKey"      orm:"idempotency_key"       description:""`
	RequestDigest       string     `json:"requestDigest"       orm:"request_digest"        description:""`
	ActorUserId         int64      `json:"actorUserId"         orm:"actor_user_id"         description:""`
	SafeNote            string     `json:"safeNote"            orm:"safe_note"             description:""`
	VerifiedAt          *time.Time `json:"verifiedAt"          orm:"verified_at"           description:""`
}
