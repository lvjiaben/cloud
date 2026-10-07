// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerLegalChangeRequest is the golang structure of table plugin_idc_core_customer_legal_change_request for DAO operations like Where/Data.
type CustomerLegalChangeRequest struct {
	g.Meta                        `orm:"table:plugin_idc_core_customer_legal_change_request, do:true"`
	Id                            any        //
	TenantId                      any        //
	RequestNo                     any        //
	CustomerNo                    any        //
	CustomerTypeCode              any        //
	StatusCode                    any        //
	ReasonCode                    any        //
	ChangeLegalName               any        // Declares a requested field category without storing the proposed raw legal value
	ChangeLegalIdentifier         any        // Declares a requested identifier category without storing a raw identifier or document
	CurrentVerificationStatusCode any        //
	CurrentVerificationCaseNo     any        // Opaque reference to the verification case whose result remains unchanged while this request is reviewed
	CurrentProfileVersion         any        //
	IdempotencyKeyDigest          any        //
	RequestFingerprint            any        //
	SubmittedByUserId             any        //
	SubmittedAt                   *time.Time //
	ReviewedByUserId              any        //
	ReviewedAt                    *time.Time //
	ResolutionCode                any        //
	CreatedAt                     *time.Time //
	UpdatedAt                     *time.Time //
	ReviewVerificationCaseNo      any        // Opaque compliance case performing this legal-data review; proposed values and materials remain compliance-owned
}
