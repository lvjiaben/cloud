// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TaxInvoiceAction is the golang structure for table tax_invoice_action.
type TaxInvoiceAction struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	ActionNo             string     `json:"actionNo"             orm:"action_no"              description:""`
	TaxInvoiceNo         string     `json:"taxInvoiceNo"         orm:"tax_invoice_no"         description:""`
	ActionCode           string     `json:"actionCode"           orm:"action_code"            description:""`
	IdempotencyDigest    string     `json:"idempotencyDigest"    orm:"idempotency_digest"     description:""`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:""`
	ActorUserId          int        `json:"actorUserId"          orm:"actor_user_id"          description:""`
	BeforeStatusCode     string     `json:"beforeStatusCode"     orm:"before_status_code"     description:""`
	AfterStatusCode      string     `json:"afterStatusCode"      orm:"after_status_code"      description:""`
	BeforeVersion        int64      `json:"beforeVersion"        orm:"before_version"         description:""`
	AfterVersion         int64      `json:"afterVersion"         orm:"after_version"          description:""`
	InvoiceTypeCode      string     `json:"invoiceTypeCode"      orm:"invoice_type_code"      description:""`
	ProviderCode         string     `json:"providerCode"         orm:"provider_code"          description:""`
	ReasonCode           string     `json:"reasonCode"           orm:"reason_code"            description:""`
	InternalNote         string     `json:"internalNote"         orm:"internal_note"          description:""`
	CapabilityReasonCode string     `json:"capabilityReasonCode" orm:"capability_reason_code" description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
}
