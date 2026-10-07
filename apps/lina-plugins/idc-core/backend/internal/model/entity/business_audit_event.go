// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BusinessAuditEvent is the golang structure for table business_audit_event.
type BusinessAuditEvent struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	AuditNo           string     `json:"auditNo"           orm:"audit_no"            description:"Opaque audit event business number"`
	OccurredAt        *time.Time `json:"occurredAt"        orm:"occurred_at"         description:""`
	ActorType         string     `json:"actorType"         orm:"actor_type"          description:"Authenticated user, system, or external actor type"`
	ActorUserId       int        `json:"actorUserId"       orm:"actor_user_id"       description:"Effective host identity responsible for the action"`
	ActingUserId      int        `json:"actingUserId"      orm:"acting_user_id"      description:"Actual host identity acting through assistance or delegation"`
	ActorAudienceCode string     `json:"actorAudienceCode" orm:"actor_audience_code" description:""`
	ActorCustomerNo   string     `json:"actorCustomerNo"   orm:"actor_customer_no"   description:"Customer subject represented by the actor when the actor audience is customer"`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"         description:"Validated customer subject affected by the audited operation"`
	OperationCode     string     `json:"operationCode"     orm:"operation_code"      description:"Stable audited business operation code"`
	TargetType        string     `json:"targetType"        orm:"target_type"         description:""`
	TargetNo          string     `json:"targetNo"          orm:"target_no"           description:"Opaque target business number"`
	ResultCode        string     `json:"resultCode"        orm:"result_code"         description:""`
	ReasonCode        string     `json:"reasonCode"        orm:"reason_code"         description:""`
	RiskCode          string     `json:"riskCode"          orm:"risk_code"           description:""`
	BeforeSnapshot    string     `json:"beforeSnapshot"    orm:"before_snapshot"     description:"Redacted state before the operation"`
	AfterSnapshot     string     `json:"afterSnapshot"     orm:"after_snapshot"      description:"Redacted state after the operation"`
	ContextSnapshot   string     `json:"contextSnapshot"   orm:"context_snapshot"    description:"Redacted permission, reason, and risk context"`
	RequestId         string     `json:"requestId"         orm:"request_id"          description:""`
	TraceId           string     `json:"traceId"           orm:"trace_id"            description:""`
	CorrelationNo     string     `json:"correlationNo"     orm:"correlation_no"      description:""`
	IpDigest          string     `json:"ipDigest"          orm:"ip_digest"           description:"Keyed digest of the source IP when policy permits"`
	UserAgentDigest   string     `json:"userAgentDigest"   orm:"user_agent_digest"   description:"Digest of the user agent when policy permits"`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
