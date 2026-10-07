// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceRenewalAttempt is the golang structure of table plugin_idc_core_service_renewal_attempt for DAO operations like Where/Data.
type ServiceRenewalAttempt struct {
	g.Meta                    `orm:"table:plugin_idc_core_service_renewal_attempt, do:true"`
	Id                        any        //
	TenantId                  any        //
	AttemptNo                 any        //
	OperationNo               any        //
	EntitlementNo             any        //
	CustomerNo                any        //
	CurrencyCode              any        //
	InvoiceNo                 any        //
	PolicyNo                  any        //
	PolicyVersion             any        //
	AttemptSequence           any        //
	OutcomeCode               any        //
	LedgerTransactionNo       *string    //
	LedgerTransactionTypeCode *string    //
	FailureCode               any        //
	NextAttemptAt             *time.Time //
	EvidenceDigest            any        //
	ContentDigest             any        //
	CreatedAt                 *time.Time //
}
