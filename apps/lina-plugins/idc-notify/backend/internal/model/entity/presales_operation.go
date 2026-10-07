// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PresalesOperation is the golang structure for table presales_operation.
type PresalesOperation struct {
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	OperationDigest         string     `json:"operationDigest"         orm:"operation_digest"          description:""`
	OperationCode           string     `json:"operationCode"           orm:"operation_code"            description:""`
	VerificationNo          string     `json:"verificationNo"          orm:"verification_no"           description:""`
	DestinationKind         string     `json:"destinationKind"         orm:"destination_kind"          description:""`
	DestinationDigest       string     `json:"destinationDigest"       orm:"destination_digest"        description:""`
	BrowserDigest           string     `json:"browserDigest"           orm:"browser_digest"            description:""`
	TicketDigest            string     `json:"ticketDigest"            orm:"ticket_digest"             description:""`
	RequestDigest           string     `json:"requestDigest"           orm:"request_digest"            description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	TicketNo                string     `json:"ticketNo"                orm:"ticket_no"                 description:""`
	AccessExpiresAt         *time.Time `json:"accessExpiresAt"         orm:"access_expires_at"         description:""`
	LinkDeliveryState       string     `json:"linkDeliveryState"       orm:"link_delivery_state"       description:""`
	ProviderBoundaryCrossed bool       `json:"providerBoundaryCrossed" orm:"provider_boundary_crossed" description:""`
	CompletedAt             *time.Time `json:"completedAt"             orm:"completed_at"              description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
}
