// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PersonalVerificationAttempt is the golang structure for table personal_verification_attempt.
type PersonalVerificationAttempt struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	AttemptNo                 string     `json:"attemptNo"                 orm:"attempt_no"                   description:""`
	CaseNo                    string     `json:"caseNo"                    orm:"case_no"                      description:""`
	SubjectTypeCode           string     `json:"subjectTypeCode"           orm:"subject_type_code"            description:""`
	ConfigurationNo           string     `json:"configurationNo"           orm:"configuration_no"             description:""`
	ProviderCode              string     `json:"providerCode"              orm:"provider_code"                description:""`
	ConfigurationVersion      int64      `json:"configurationVersion"      orm:"configuration_version"        description:""`
	OuterOrderNo              string     `json:"outerOrderNo"              orm:"outer_order_no"               description:""`
	CertifyId                 *string    `json:"certifyId"                 orm:"certify_id"                   description:"Aliyun billing and reconciliation identifier; certify URL, face images, face videos, identity plaintext, and raw provider payloads are forbidden"`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                  description:""`
	LastProviderResultCode    string     `json:"lastProviderResultCode"    orm:"last_provider_result_code"    description:""`
	NotPassedObservationCount int        `json:"notPassedObservationCount" orm:"not_passed_observation_count" description:""`
	InitializationStartedAt   *time.Time `json:"initializationStartedAt"   orm:"initialization_started_at"    description:""`
	IssuedAt                  *time.Time `json:"issuedAt"                  orm:"issued_at"                    description:""`
	CertifyExpiresAt          *time.Time `json:"certifyExpiresAt"          orm:"certify_expires_at"           description:""`
	LastQueriedAt             *time.Time `json:"lastQueriedAt"             orm:"last_queried_at"              description:""`
	PassedAt                  *time.Time `json:"passedAt"                  orm:"passed_at"                    description:""`
	ExpiredAt                 *time.Time `json:"expiredAt"                 orm:"expired_at"                   description:""`
	Version                   int64      `json:"version"                   orm:"version"                      description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
	UpdatedAt                 *time.Time `json:"updatedAt"                 orm:"updated_at"                   description:""`
}
