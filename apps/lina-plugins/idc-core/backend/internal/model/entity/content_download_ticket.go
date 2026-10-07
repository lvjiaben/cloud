// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentDownloadTicket is the golang structure for table content_download_ticket.
type ContentDownloadTicket struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int64      `json:"tenantId"               orm:"tenant_id"                description:""`
	TicketNo               string     `json:"ticketNo"               orm:"ticket_no"                description:""`
	TokenDigest            string     `json:"tokenDigest"            orm:"token_digest"             description:""`
	ContentNo              string     `json:"contentNo"              orm:"content_no"               description:""`
	FileNo                 string     `json:"fileNo"                 orm:"file_no"                  description:""`
	ContentValidityVersion int64      `json:"contentValidityVersion" orm:"content_validity_version" description:""`
	VisibilityCode         string     `json:"visibilityCode"         orm:"visibility_code"          description:""`
	UserId                 int64      `json:"userId"                 orm:"user_id"                  description:""`
	SessionReferenceDigest string     `json:"sessionReferenceDigest" orm:"session_reference_digest" description:""`
	AuthorizationRevision  int64      `json:"authorizationRevision"  orm:"authorization_revision"   description:""`
	IssuedAt               *time.Time `json:"issuedAt"               orm:"issued_at"                description:""`
	ExpiresAt              *time.Time `json:"expiresAt"              orm:"expires_at"               description:""`
	RevokedAt              *time.Time `json:"revokedAt"              orm:"revoked_at"               description:""`
}
