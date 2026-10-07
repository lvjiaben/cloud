// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketAccessLink is the golang structure for table support_ticket_access_link.
type SupportTicketAccessLink struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int64      `json:"tenantId"          orm:"tenant_id"          description:""`
	LinkNo            string     `json:"linkNo"            orm:"link_no"            description:""`
	TicketNo          string     `json:"ticketNo"          orm:"ticket_no"          description:""`
	TokenDigest       string     `json:"tokenDigest"       orm:"token_digest"       description:""`
	DestinationDigest string     `json:"destinationDigest" orm:"destination_digest" description:""`
	DestinationKind   string     `json:"destinationKind"   orm:"destination_kind"   description:""`
	OperationDigest   string     `json:"operationDigest"   orm:"operation_digest"   description:""`
	RequestDigest     string     `json:"requestDigest"     orm:"request_digest"     description:""`
	TokenSecretRef    string     `json:"tokenSecretRef"    orm:"token_secret_ref"   description:""`
	ExpiresAt         *time.Time `json:"expiresAt"         orm:"expires_at"         description:""`
	RevokedAt         *time.Time `json:"revokedAt"         orm:"revoked_at"         description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
}
