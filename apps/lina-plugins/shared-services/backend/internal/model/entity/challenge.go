// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Challenge is the golang structure for table challenge.
type Challenge struct {
	Id               int64      `json:"id"               orm:"id"                description:""`
	TenantId         int64      `json:"tenantId"         orm:"tenant_id"         description:""`
	ChallengeNo      string     `json:"challengeNo"      orm:"challenge_no"      description:""`
	CallerPluginId   string     `json:"callerPluginId"   orm:"caller_plugin_id"  description:""`
	Purpose          string     `json:"purpose"          orm:"purpose"           description:""`
	BindingDigest    string     `json:"bindingDigest"    orm:"binding_digest"    description:""`
	ProviderCode     string     `json:"providerCode"     orm:"provider_code"     description:""`
	PresentationKind string     `json:"presentationKind" orm:"presentation_kind" description:""`
	AnswerSalt       any        `json:"answerSalt"       orm:"answer_salt"       description:""`
	AnswerDigest     any        `json:"answerDigest"     orm:"answer_digest"     description:""`
	ProofDigest      any        `json:"proofDigest"      orm:"proof_digest"      description:""`
	Status           string     `json:"status"           orm:"status"            description:""`
	AttemptCount     int        `json:"attemptCount"     orm:"attempt_count"     description:""`
	MaxAttempts      int        `json:"maxAttempts"      orm:"max_attempts"      description:""`
	ExpiresAt        *time.Time `json:"expiresAt"        orm:"expires_at"        description:""`
	ProofExpiresAt   any        `json:"proofExpiresAt"   orm:"proof_expires_at"  description:""`
	VerifiedAt       any        `json:"verifiedAt"       orm:"verified_at"       description:""`
	ConsumedAt       any        `json:"consumedAt"       orm:"consumed_at"       description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"        description:""`
}
