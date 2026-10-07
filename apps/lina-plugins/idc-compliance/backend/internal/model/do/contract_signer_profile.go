// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractSignerProfile is the golang structure of table plugin_idc_compliance_contract_signer_profile for DAO operations like Where/Data.
type ContractSignerProfile struct {
	g.Meta                      `orm:"table:plugin_idc_compliance_contract_signer_profile, do:true"`
	Id                          any        //
	TenantId                    any        //
	SignerNo                    any        //
	CustomerNo                  any        //
	ProfileSnapshotRef          any        //
	ProfileSnapshotSha256       any        //
	MaskedName                  any        //
	MaskedIdentityNo            any        //
	MaskedPhone                 any        //
	ConsentPolicyVersion        any        //
	ConsentOccurredAt           *time.Time //
	IdentityStatusCode          any        //
	IdentityAttemptNo           *string    //
	IdentityVerificationVersion any        //
	IdentityVerifiedAt          *time.Time //
	PhoneStatusCode             any        //
	PhoneVerificationNo         *string    //
	PhoneVerificationVersion    any        //
	PhoneVerifiedAt             *time.Time //
	StatusCode                  any        //
	AuthorizationVersion        any        //
	CreationDigest              any        //
	DisabledAt                  *time.Time //
	Version                     any        //
	CreatedAt                   *time.Time //
	UpdatedAt                   *time.Time //
}
