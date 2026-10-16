// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerIndividualProfile is the golang structure for table customer_individual_profile.
type CustomerIndividualProfile struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	CustomerNo               string     `json:"customerNo"               orm:"customer_no"                description:""`
	CustomerTypeCode         string     `json:"customerTypeCode"         orm:"customer_type_code"         description:""`
	LegalName                string     `json:"legalName"                orm:"legal_name"                 description:"Legal name protected by customer-profile access controls"`
	LegalNameMasked          string     `json:"legalNameMasked"          orm:"legal_name_masked"          description:""`
	IdentityIdentifierMasked string     `json:"identityIdentifierMasked" orm:"identity_identifier_masked" description:"Display-safe masked identity identifier; the raw identifier is not stored in this table"`
	IdentityLookupDigest     string     `json:"identityLookupDigest"     orm:"identity_lookup_digest"     description:"Keyed lookup digest; never a raw identity number or unsalted public hash"`
	IdentityCaseNo           string     `json:"identityCaseNo"           orm:"identity_case_no"           description:"Opaque compliance-owned verification case reference without a database foreign key"`
	IdentityStatusCode       string     `json:"identityStatusCode"       orm:"identity_status_code"       description:""`
	IdentityVerifiedAt       *time.Time `json:"identityVerifiedAt"       orm:"identity_verified_at"       description:""`
	IdentityExpiresAt        *time.Time `json:"identityExpiresAt"        orm:"identity_expires_at"        description:""`
	Version                  int64      `json:"version"                  orm:"version"                    description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                 description:""`
	ManualVerification       bool       `json:"manualVerification"       orm:"manual_verification"        description:"Current verification status set by staff; not supplier identity evidence"`
}
