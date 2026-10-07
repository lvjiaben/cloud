// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialExportTicket is the golang structure for table financial_export_ticket.
type FinancialExportTicket struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	TicketNo       string     `json:"ticketNo"       orm:"ticket_no"       description:""`
	TokenDigest    string     `json:"tokenDigest"    orm:"token_digest"    description:""`
	RequestNo      string     `json:"requestNo"      orm:"request_no"      description:""`
	RequestedBy    int        `json:"requestedBy"    orm:"requested_by"    description:""`
	PermissionCode string     `json:"permissionCode" orm:"permission_code" description:""`
	DataScopeCode  int        `json:"dataScopeCode"  orm:"data_scope_code" description:""`
	ExpiresAt      *time.Time `json:"expiresAt"      orm:"expires_at"      description:""`
	ConsumedAt     *time.Time `json:"consumedAt"     orm:"consumed_at"     description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
