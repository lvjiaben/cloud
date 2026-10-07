// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionPayeeProfile is the golang structure for table sales_commission_payee_profile.
type SalesCommissionPayeeProfile struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	ProfileNo                string     `json:"profileNo"                orm:"profile_no"                 description:""`
	BeneficiaryTypeCode      string     `json:"beneficiaryTypeCode"      orm:"beneficiary_type_code"      description:""`
	BeneficiaryReference     string     `json:"beneficiaryReference"     orm:"beneficiary_reference"      description:""`
	PayeeTypeCode            string     `json:"payeeTypeCode"            orm:"payee_type_code"            description:""`
	ProtectedObjectReference string     `json:"protectedObjectReference" orm:"protected_object_reference" description:""`
	AccountDigest            string     `json:"accountDigest"            orm:"account_digest"             description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                description:""`
	Version                  int64      `json:"version"                  orm:"version"                    description:""`
	VerifiedAt               *time.Time `json:"verifiedAt"               orm:"verified_at"                description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                 description:""`
}
