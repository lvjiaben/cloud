// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketUpstreamForward is the golang structure of table plugin_idc_core_support_ticket_upstream_forward for DAO operations like Where/Data.
type SupportTicketUpstreamForward struct {
	g.Meta                  `orm:"table:plugin_idc_core_support_ticket_upstream_forward, do:true"`
	Id                      any        //
	TenantId                any        //
	ForwardNo               any        //
	TicketNo                any        //
	ProviderId              any        //
	ProviderTicketReference any        //
	SelectionDigest         any        //
	PreviewDigest           any        //
	StatusCode              any        //
	ResultClassification    any        //
	RetryEligible           any        //
	ConfirmedBy             any        //
	ConfirmedAt             *time.Time //
	Version                 any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
	OperationNo             any        //
	RequestDigest           any        //
	UpstreamDepartmentId    any        //
	EntitlementUpstreamRef  any        //
	AttemptCount            any        //
	LastAttemptAt           *time.Time //
	NextQueryAt             *time.Time //
	ConfirmedReason         any        //
	LastEvidenceCode        any        //
}
