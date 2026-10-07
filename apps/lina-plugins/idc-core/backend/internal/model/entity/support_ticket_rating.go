// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketRating is the golang structure for table support_ticket_rating.
type SupportTicketRating struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int64      `json:"tenantId"   orm:"tenant_id"   description:""`
	RatingNo   string     `json:"ratingNo"   orm:"rating_no"   description:""`
	TicketNo   string     `json:"ticketNo"   orm:"ticket_no"   description:""`
	RoundNo    int        `json:"roundNo"    orm:"round_no"    description:""`
	Score      int        `json:"score"      orm:"score"       description:""`
	Comment    string     `json:"comment"    orm:"comment"     description:""`
	AuthorType string     `json:"authorType" orm:"author_type" description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
}
