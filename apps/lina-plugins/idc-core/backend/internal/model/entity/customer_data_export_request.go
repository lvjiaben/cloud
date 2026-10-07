// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerDataExportRequest is the golang structure for table customer_data_export_request.
type CustomerDataExportRequest struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	RequestNo            string     `json:"requestNo"            orm:"request_no"             description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"            description:""`
	ConsentNo            string     `json:"consentNo"            orm:"consent_no"             description:""`
	TaskNo               string     `json:"taskNo"               orm:"task_no"                description:"Application-validated asynchronous task reference without a cross-domain database foreign key"`
	ScopeCode            string     `json:"scopeCode"            orm:"scope_code"             description:""`
	FormatCode           string     `json:"formatCode"           orm:"format_code"            description:""`
	SubjectVersion       int64      `json:"subjectVersion"       orm:"subject_version"        description:""`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest" description:""`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:""`
	ReauthIssuedAt       *time.Time `json:"reauthIssuedAt"       orm:"reauth_issued_at"       description:""`
	RequestedByUserId    int        `json:"requestedByUserId"    orm:"requested_by_user_id"   description:""`
	RequestedAt          *time.Time `json:"requestedAt"          orm:"requested_at"           description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
