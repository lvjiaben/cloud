// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicket is the golang structure for table support_ticket.
type SupportTicket struct {
	Id                         int64      `json:"id"                         orm:"id"                             description:""`
	TenantId                   int64      `json:"tenantId"                   orm:"tenant_id"                      description:""`
	TicketNo                   string     `json:"ticketNo"                   orm:"ticket_no"                      description:""`
	TicketType                 string     `json:"ticketType"                 orm:"ticket_type"                    description:""`
	VisibilityCode             string     `json:"visibilityCode"             orm:"visibility_code"                description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                    description:""`
	RelatedEntitlementNo       string     `json:"relatedEntitlementNo"       orm:"related_entitlement_no"         description:"Immutable optional customer service reference revalidated at creation; intentionally not a foreign key so ticket history survives service archival"`
	RelatedEntitlementVersion  int64      `json:"relatedEntitlementVersion"  orm:"related_entitlement_version"    description:""`
	RelatedEntitlementNameZhCn string     `json:"relatedEntitlementNameZhCn" orm:"related_entitlement_name_zh_cn" description:""`
	RelatedEntitlementNameEnUs string     `json:"relatedEntitlementNameEnUs" orm:"related_entitlement_name_en_us" description:""`
	RelatedEntitlementStatus   string     `json:"relatedEntitlementStatus"   orm:"related_entitlement_status"     description:""`
	AnonymousDestinationDigest string     `json:"anonymousDestinationDigest" orm:"anonymous_destination_digest"   description:""`
	AnonymousDestinationKind   string     `json:"anonymousDestinationKind"   orm:"anonymous_destination_kind"     description:""`
	CategoryNo                 string     `json:"categoryNo"                 orm:"category_no"                    description:""`
	DepartmentNo               string     `json:"departmentNo"               orm:"department_no"                  description:""`
	AssigneeUserId             int64      `json:"assigneeUserId"             orm:"assignee_user_id"               description:""`
	Subject                    string     `json:"subject"                    orm:"subject"                        description:""`
	BaseStage                  string     `json:"baseStage"                  orm:"base_stage"                     description:""`
	CustomStatusNo             string     `json:"customStatusNo"             orm:"custom_status_no"               description:""`
	PriorityCode               string     `json:"priorityCode"               orm:"priority_code"                  description:""`
	CurrentRound               int        `json:"currentRound"               orm:"current_round"                  description:""`
	FollowUpRequired           bool       `json:"followUpRequired"           orm:"follow_up_required"             description:""`
	SlaRiskCode                string     `json:"slaRiskCode"                orm:"sla_risk_code"                  description:""`
	ResolvedAt                 *time.Time `json:"resolvedAt"                 orm:"resolved_at"                    description:""`
	ClosedAt                   *time.Time `json:"closedAt"                   orm:"closed_at"                      description:""`
	ClosedReasonCode           string     `json:"closedReasonCode"           orm:"closed_reason_code"             description:""`
	ClosedByType               string     `json:"closedByType"               orm:"closed_by_type"                 description:""`
	CustomerLastRepliedAt      *time.Time `json:"customerLastRepliedAt"      orm:"customer_last_replied_at"       description:""`
	LastActivityAt             *time.Time `json:"lastActivityAt"             orm:"last_activity_at"               description:""`
	Version                    int64      `json:"version"                    orm:"version"                        description:""`
	CreatedBy                  int64      `json:"createdBy"                  orm:"created_by"                     description:""`
	UpdatedBy                  int64      `json:"updatedBy"                  orm:"updated_by"                     description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                     description:""`
	UpdatedAt                  *time.Time `json:"updatedAt"                  orm:"updated_at"                     description:""`
}
