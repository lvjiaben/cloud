// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProvisionOutcomeReceipt is the golang structure for table provision_outcome_receipt.
type ProvisionOutcomeReceipt struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	ReceiptNo               string     `json:"receiptNo"               orm:"receipt_no"                description:""`
	TaskNo                  string     `json:"taskNo"                  orm:"task_no"                   description:""`
	ServiceNo               string     `json:"serviceNo"               orm:"service_no"                description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	RequestDigest           string     `json:"requestDigest"           orm:"request_digest"            description:""`
	TaskEvidenceDigest      string     `json:"taskEvidenceDigest"      orm:"task_evidence_digest"      description:""`
	CoreEvidenceDigest      string     `json:"coreEvidenceDigest"      orm:"core_evidence_digest"      description:"Secret-free evidence produced after core commits its service and fulfillment state"`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	OperationNo             string     `json:"operationNo"             orm:"operation_no"              description:"Frozen opaque fulfillment operation copied from the validated provision request"`
	CustomerNo              string     `json:"customerNo"              orm:"customer_no"               description:"Frozen opaque customer owner copied from the validated provision request"`
	ConfigurationSnapshotNo string     `json:"configurationSnapshotNo" orm:"configuration_snapshot_no" description:"Frozen configuration snapshot copied from the validated provision request"`
}
