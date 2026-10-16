// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerIndividualProfile is the golang structure of table plugin_idc_core_customer_individual_profile for DAO operations like Where/Data.
type CustomerIndividualProfile struct {
	g.Meta                   `orm:"table:plugin_idc_core_customer_individual_profile, do:true"`
	Id                       any        //
	TenantId                 any        //
	CustomerNo               any        //
	CustomerTypeCode         any        //
	LegalName                any        // Legal name protected by customer-profile access controls
	LegalNameMasked          any        //
	IdentityIdentifierMasked any        // Display-safe masked identity identifier; the raw identifier is not stored in this table
	IdentityLookupDigest     any        // Keyed lookup digest; never a raw identity number or unsalted public hash
	IdentityCaseNo           any        // Opaque compliance-owned verification case reference without a database foreign key
	IdentityStatusCode       any        //
	IdentityVerifiedAt       *time.Time //
	IdentityExpiresAt        *time.Time //
	Version                  any        //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
	ManualVerification       any        // Current verification status set by staff; not supplier identity evidence
}
