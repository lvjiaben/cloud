// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerEnterpriseProfile is the golang structure for table customer_enterprise_profile.
type CustomerEnterpriseProfile struct {
	Id                           int64      `json:"id"                           orm:"id"                             description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                      description:""`
	CustomerNo                   string     `json:"customerNo"                   orm:"customer_no"                    description:""`
	CustomerTypeCode             string     `json:"customerTypeCode"             orm:"customer_type_code"             description:""`
	LegalName                    string     `json:"legalName"                    orm:"legal_name"                     description:""`
	RegistrationCountryCode      string     `json:"registrationCountryCode"      orm:"registration_country_code"      description:""`
	RegistrationIdentifier       string     `json:"registrationIdentifier"       orm:"registration_identifier"        description:"Enterprise registration identifier protected as customer legal profile data"`
	RegistrationIdentifierMasked string     `json:"registrationIdentifierMasked" orm:"registration_identifier_masked" description:""`
	RegistrationLookupDigest     string     `json:"registrationLookupDigest"     orm:"registration_lookup_digest"     description:"Keyed lookup digest used for duplicate review without query-time normalization"`
	IndustryCode                 string     `json:"industryCode"                 orm:"industry_code"                  description:""`
	VerificationCaseNo           string     `json:"verificationCaseNo"           orm:"verification_case_no"           description:"Opaque compliance-owned verification case reference without a database foreign key"`
	VerificationStatusCode       string     `json:"verificationStatusCode"       orm:"verification_status_code"       description:""`
	VerifiedAt                   *time.Time `json:"verifiedAt"                   orm:"verified_at"                    description:""`
	ExpiresAt                    *time.Time `json:"expiresAt"                    orm:"expires_at"                     description:""`
	Version                      int64      `json:"version"                      orm:"version"                        description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                     description:""`
	UpdatedAt                    *time.Time `json:"updatedAt"                    orm:"updated_at"                     description:""`
}
