// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicket is the golang structure of table plugin_idc_core_support_ticket for DAO operations like Where/Data.
type SupportTicket struct {
	g.Meta                     `orm:"table:plugin_idc_core_support_ticket, do:true"`
	Id                         any        //
	TenantId                   any        //
	TicketNo                   any        //
	TicketType                 any        //
	VisibilityCode             any        //
	CustomerNo                 any        //
	RelatedEntitlementNo       any        // Immutable optional customer service reference revalidated at creation; intentionally not a foreign key so ticket history survives service archival
	RelatedEntitlementVersion  any        //
	RelatedEntitlementNameZhCn any        //
	RelatedEntitlementNameEnUs any        //
	RelatedEntitlementStatus   any        //
	AnonymousDestinationDigest any        //
	AnonymousDestinationKind   any        //
	CategoryNo                 any        //
	DepartmentNo               any        //
	AssigneeUserId             any        //
	Subject                    any        //
	BaseStage                  any        //
	CustomStatusNo             any        //
	PriorityCode               any        //
	CurrentRound               any        //
	FollowUpRequired           any        //
	SlaRiskCode                any        //
	ResolvedAt                 *time.Time //
	ClosedAt                   *time.Time //
	ClosedReasonCode           any        //
	ClosedByType               any        //
	CustomerLastRepliedAt      *time.Time //
	LastActivityAt             *time.Time //
	Version                    any        //
	CreatedBy                  any        //
	UpdatedBy                  any        //
	CreatedAt                  *time.Time //
	UpdatedAt                  *time.Time //
}
