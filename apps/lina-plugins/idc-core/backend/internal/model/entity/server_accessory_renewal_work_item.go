// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerAccessoryRenewalWorkItem is the golang structure for table server_accessory_renewal_work_item.
type ServerAccessoryRenewalWorkItem struct {
	Id                 int64      `json:"id"                 orm:"id"                  description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"           description:""`
	WorkItemNo         string     `json:"workItemNo"         orm:"work_item_no"        description:""`
	EntitlementNo      string     `json:"entitlementNo"      orm:"entitlement_no"      description:""`
	CustomerNo         string     `json:"customerNo"         orm:"customer_no"         description:""`
	ServiceNo          string     `json:"serviceNo"          orm:"service_no"          description:""`
	InvoiceNo          string     `json:"invoiceNo"          orm:"invoice_no"          description:""`
	RequiredPermission string     `json:"requiredPermission" orm:"required_permission" description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"         description:""`
	ReasonCode         string     `json:"reasonCode"         orm:"reason_code"         description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"          description:""`
	ResolvedAt         *time.Time `json:"resolvedAt"         orm:"resolved_at"         description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"          description:""`
	SubjectTypeCode    string     `json:"subjectTypeCode"    orm:"subject_type_code"   description:""`
	SubjectNo          string     `json:"subjectNo"          orm:"subject_no"          description:""`
}
