// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterTask is the golang structure of table plugin_idc_compliance_red_letter_task for DAO operations like Where/Data.
type RedLetterTask struct {
	g.Meta                       `orm:"table:plugin_idc_compliance_red_letter_task, do:true"`
	Id                           any        //
	TenantId                     any        //
	TaskNo                       any        //
	TaxInvoiceNo                 *string    //
	RequestSha256                any        //
	RefundOperationNo            any        //
	ReasonCode                   any        //
	StatusCode                   any        //
	GrossAmountMinor             any        //
	NetAmountMinor               any        //
	TaxAmountMinor               any        //
	CustomerConfirmationRequired any        //
	CustomerConfirmedAt          *time.Time //
	ProviderCode                 *string    //
	ExternalTaskRef              *string    //
	AvailableAt                  *time.Time //
	NextQueryAt                  *time.Time //
	CompletedAt                  *time.Time //
	FailureCode                  any        //
	Version                      any        //
	CreatedAt                    *time.Time //
	UpdatedAt                    *time.Time //
	CustomerNo                   any        //
	CurrencyCode                 any        //
	AllocationResolved           any        //
	ManualReasonCode             any        //
	CompletionEvidenceNo         *string    //
	CustomerRecheckRequestedAt   *time.Time //
}
