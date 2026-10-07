// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractSignerIdentityAttempt is the golang structure for table contract_signer_identity_attempt.
type ContractSignerIdentityAttempt struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	AttemptNo               string     `json:"attemptNo"               orm:"attempt_no"                description:""`
	SignerNo                string     `json:"signerNo"                orm:"signer_no"                 description:""`
	OuterOrderNo            string     `json:"outerOrderNo"            orm:"outer_order_no"            description:""`
	ConfigurationNo         string     `json:"configurationNo"         orm:"configuration_no"          description:""`
	ConfigurationVersion    int64      `json:"configurationVersion"    orm:"configuration_version"     description:""`
	CredentialGeneration    int64      `json:"credentialGeneration"    orm:"credential_generation"     description:""`
	CertifyId               *string    `json:"certifyId"               orm:"certify_id"                description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	ReasonCode              string     `json:"reasonCode"              orm:"reason_code"               description:""`
	InitializationStartedAt *time.Time `json:"initializationStartedAt" orm:"initialization_started_at" description:""`
	CertifyExpiresAt        *time.Time `json:"certifyExpiresAt"        orm:"certify_expires_at"        description:""`
	LastQueriedAt           *time.Time `json:"lastQueriedAt"           orm:"last_queried_at"           description:""`
	PassedAt                *time.Time `json:"passedAt"                orm:"passed_at"                 description:""`
	ExpiredAt               *time.Time `json:"expiredAt"               orm:"expired_at"                description:""`
	Version                 int64      `json:"version"                 orm:"version"                   description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
}
