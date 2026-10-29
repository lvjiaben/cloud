// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// NativeAccessoryReceipt is the golang structure for table native_accessory_receipt.
type NativeAccessoryReceipt struct {
	TaskNo        string     `json:"taskNo"        orm:"task_no"        description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	OperationNo   string     `json:"operationNo"   orm:"operation_no"   description:""`
	EntitlementNo string     `json:"entitlementNo" orm:"entitlement_no" description:""`
	CustomerNo    string     `json:"customerNo"    orm:"customer_no"    description:""`
	ObjectNo      string     `json:"objectNo"      orm:"object_no"      description:""`
	AccessoryKind string     `json:"accessoryKind" orm:"accessory_kind" description:""`
	NextReportAt  *time.Time `json:"nextReportAt"  orm:"next_report_at" description:""`
	CompletedAt   *time.Time `json:"completedAt"   orm:"completed_at"   description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"     description:""`
	UpdatedAt     *time.Time `json:"updatedAt"     orm:"updated_at"     description:""`
}
