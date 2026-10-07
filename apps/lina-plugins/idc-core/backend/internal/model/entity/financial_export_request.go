// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialExportRequest is the golang structure for table financial_export_request.
type FinancialExportRequest struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	RequestNo              string     `json:"requestNo"              orm:"request_no"               description:""`
	ExportTypeCode         string     `json:"exportTypeCode"         orm:"export_type_code"         description:""`
	FilterJson             string     `json:"filterJson"             orm:"filter_json"              description:""`
	FilterDigest           string     `json:"filterDigest"           orm:"filter_digest"            description:""`
	PermissionCode         string     `json:"permissionCode"         orm:"permission_code"          description:""`
	DataScopeCode          int        `json:"dataScopeCode"          orm:"data_scope_code"          description:""`
	RequestedBy            int        `json:"requestedBy"            orm:"requested_by"             description:""`
	IdempotencyDigest      string     `json:"idempotencyDigest"      orm:"idempotency_digest"       description:""`
	AuthorizationCheckedAt *time.Time `json:"authorizationCheckedAt" orm:"authorization_checked_at" description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"              description:""`
	FailureCode            string     `json:"failureCode"            orm:"failure_code"             description:""`
	GeneratedAt            *time.Time `json:"generatedAt"            orm:"generated_at"             description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
}
