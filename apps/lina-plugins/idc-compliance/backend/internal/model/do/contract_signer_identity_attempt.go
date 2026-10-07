// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractSignerIdentityAttempt is the golang structure of table plugin_idc_compliance_contract_signer_identity_attempt for DAO operations like Where/Data.
type ContractSignerIdentityAttempt struct {
	g.Meta                  `orm:"table:plugin_idc_compliance_contract_signer_identity_attempt, do:true"`
	Id                      any        //
	TenantId                any        //
	AttemptNo               any        //
	SignerNo                any        //
	OuterOrderNo            any        //
	ConfigurationNo         any        //
	ConfigurationVersion    any        //
	CredentialGeneration    any        //
	CertifyId               *string    //
	StatusCode              any        //
	ReasonCode              any        //
	InitializationStartedAt *time.Time //
	CertifyExpiresAt        *time.Time //
	LastQueriedAt           *time.Time //
	PassedAt                *time.Time //
	ExpiredAt               *time.Time //
	Version                 any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
