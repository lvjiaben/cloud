// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketUpstreamForward is the golang structure for table support_ticket_upstream_forward.
type SupportTicketUpstreamForward struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int64      `json:"tenantId"                orm:"tenant_id"                 description:""`
	ForwardNo               string     `json:"forwardNo"               orm:"forward_no"                description:""`
	TicketNo                string     `json:"ticketNo"                orm:"ticket_no"                 description:""`
	ProviderId              string     `json:"providerId"              orm:"provider_id"               description:""`
	ProviderTicketReference string     `json:"providerTicketReference" orm:"provider_ticket_reference" description:""`
	SelectionDigest         string     `json:"selectionDigest"         orm:"selection_digest"          description:""`
	PreviewDigest           string     `json:"previewDigest"           orm:"preview_digest"            description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	ResultClassification    string     `json:"resultClassification"    orm:"result_classification"     description:""`
	RetryEligible           bool       `json:"retryEligible"           orm:"retry_eligible"            description:""`
	ConfirmedBy             int64      `json:"confirmedBy"             orm:"confirmed_by"              description:""`
	ConfirmedAt             *time.Time `json:"confirmedAt"             orm:"confirmed_at"              description:""`
	Version                 int64      `json:"version"                 orm:"version"                   description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
	OperationNo             string     `json:"operationNo"             orm:"operation_no"              description:""`
	RequestDigest           string     `json:"requestDigest"           orm:"request_digest"            description:""`
	UpstreamDepartmentId    int64      `json:"upstreamDepartmentId"    orm:"upstream_department_id"    description:""`
	EntitlementUpstreamRef  string     `json:"entitlementUpstreamRef"  orm:"entitlement_upstream_ref"  description:""`
	AttemptCount            int        `json:"attemptCount"            orm:"attempt_count"             description:""`
	LastAttemptAt           *time.Time `json:"lastAttemptAt"           orm:"last_attempt_at"           description:""`
	NextQueryAt             *time.Time `json:"nextQueryAt"             orm:"next_query_at"             description:""`
	ConfirmedReason         string     `json:"confirmedReason"         orm:"confirmed_reason"          description:""`
	LastEvidenceCode        string     `json:"lastEvidenceCode"        orm:"last_evidence_code"        description:""`
}
