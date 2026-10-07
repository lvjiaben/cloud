// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerClosureRequest is the golang structure of table plugin_idc_core_customer_closure_request for DAO operations like Where/Data.
type CustomerClosureRequest struct {
	g.Meta                   `orm:"table:plugin_idc_core_customer_closure_request, do:true"`
	Id                       any        //
	TenantId                 any        //
	RequestNo                any        //
	CustomerNo               any        //
	ConsentNo                any        //
	StatusCode               any        //
	SubjectVersion           any        //
	UnsettledFundsCount      any        //
	ActiveServiceCount       any        //
	UnsettledInvoiceCount    any        //
	ActiveContractCount      any        //
	InProgressOperationCount any        //
	LegalRetentionCount      any        // Retention findings do not by themselves authorize deletion or bypass closure review; legally retained evidence survives ordinary-profile anonymization
	PrecheckContractVersion  any        //
	PrecheckedAt             *time.Time //
	RetentionPolicyCode      any        //
	AnonymizationPolicyCode  any        //
	IdempotencyKeyDigest     any        //
	RequestFingerprint       any        //
	RequestedByUserId        any        //
	ConfirmedAt              *time.Time //
	RequestedAt              *time.Time //
	ReviewedByUserId         any        //
	ReviewedAt               *time.Time //
	CompletedAt              *time.Time //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
