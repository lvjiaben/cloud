// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerEnterpriseProfile is the golang structure of table plugin_idc_core_customer_enterprise_profile for DAO operations like Where/Data.
type CustomerEnterpriseProfile struct {
	g.Meta                       `orm:"table:plugin_idc_core_customer_enterprise_profile, do:true"`
	Id                           any        //
	TenantId                     any        //
	CustomerNo                   any        //
	CustomerTypeCode             any        //
	LegalName                    any        //
	RegistrationCountryCode      any        //
	RegistrationIdentifier       any        // Enterprise registration identifier protected as customer legal profile data
	RegistrationIdentifierMasked any        //
	RegistrationLookupDigest     any        // Keyed lookup digest used for duplicate review without query-time normalization
	IndustryCode                 any        //
	VerificationCaseNo           any        // Opaque compliance-owned verification case reference without a database foreign key
	VerificationStatusCode       any        //
	VerifiedAt                   *time.Time //
	ExpiresAt                    *time.Time //
	Version                      any        //
	CreatedAt                    *time.Time //
	UpdatedAt                    *time.Time //
}
