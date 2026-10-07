// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PersonalVerificationAttempt is the golang structure of table plugin_idc_compliance_personal_verification_attempt for DAO operations like Where/Data.
type PersonalVerificationAttempt struct {
	g.Meta                    `orm:"table:plugin_idc_compliance_personal_verification_attempt, do:true"`
	Id                        any        //
	TenantId                  any        //
	AttemptNo                 any        //
	CaseNo                    any        //
	SubjectTypeCode           any        //
	ConfigurationNo           any        //
	ProviderCode              any        //
	ConfigurationVersion      any        //
	OuterOrderNo              any        //
	CertifyId                 *string    // Aliyun billing and reconciliation identifier; certify URL, face images, face videos, identity plaintext, and raw provider payloads are forbidden
	StatusCode                any        //
	LastProviderResultCode    any        //
	NotPassedObservationCount any        //
	InitializationStartedAt   *time.Time //
	IssuedAt                  *time.Time //
	CertifyExpiresAt          *time.Time //
	LastQueriedAt             *time.Time //
	PassedAt                  *time.Time //
	ExpiredAt                 *time.Time //
	Version                   any        //
	CreatedAt                 *time.Time //
	UpdatedAt                 *time.Time //
}
