// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommercePaymentContext is the golang structure for table commerce_payment_context.
type CommercePaymentContext struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	ContextNo          string     `json:"contextNo"          orm:"context_no"           description:""`
	OrderNo            string     `json:"orderNo"            orm:"order_no"             description:""`
	InvoiceNo          string     `json:"invoiceNo"          orm:"invoice_no"           description:""`
	CustomerNo         string     `json:"customerNo"         orm:"customer_no"          description:""`
	CustomerTypeCode   string     `json:"customerTypeCode"   orm:"customer_type_code"   description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	ReasonCode         string     `json:"reasonCode"         orm:"reason_code"          description:""`
	IdentityStatusCode string     `json:"identityStatusCode" orm:"identity_status_code" description:""`
	IdentityCaseNo     string     `json:"identityCaseNo"     orm:"identity_case_no"     description:"Opaque compliance-owned case reference from the most recent evaluation, never raw identity material"`
	IdentityVersion    int64      `json:"identityVersion"    orm:"identity_version"     description:""`
	FirstRequestedAt   *time.Time `json:"firstRequestedAt"   orm:"first_requested_at"   description:""`
	LastEvaluatedAt    *time.Time `json:"lastEvaluatedAt"    orm:"last_evaluated_at"    description:""`
	ReadyAt            *time.Time `json:"readyAt"            orm:"ready_at"             description:""`
	ExpiresAt          *time.Time `json:"expiresAt"          orm:"expires_at"           description:""`
	Version            int64      `json:"version"            orm:"version"              description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"           description:""`
}
