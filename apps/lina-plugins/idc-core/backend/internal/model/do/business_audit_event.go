// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BusinessAuditEvent is the golang structure of table plugin_idc_core_business_audit_event for DAO operations like Where/Data.
type BusinessAuditEvent struct {
	g.Meta            `orm:"table:plugin_idc_core_business_audit_event, do:true"`
	Id                any        //
	TenantId          any        //
	AuditNo           any        // Opaque audit event business number
	OccurredAt        *time.Time //
	ActorType         any        // Authenticated user, system, or external actor type
	ActorUserId       any        // Effective host identity responsible for the action
	ActingUserId      any        // Actual host identity acting through assistance or delegation
	ActorAudienceCode any        //
	ActorCustomerNo   any        // Customer subject represented by the actor when the actor audience is customer
	CustomerNo        any        // Validated customer subject affected by the audited operation
	OperationCode     any        // Stable audited business operation code
	TargetType        any        //
	TargetNo          any        // Opaque target business number
	ResultCode        any        //
	ReasonCode        any        //
	RiskCode          any        //
	BeforeSnapshot    any        // Redacted state before the operation
	AfterSnapshot     any        // Redacted state after the operation
	ContextSnapshot   any        // Redacted permission, reason, and risk context
	RequestId         any        //
	TraceId           any        //
	CorrelationNo     any        //
	IpDigest          any        // Keyed digest of the source IP when policy permits
	UserAgentDigest   any        // Digest of the user agent when policy permits
	CreatedAt         *time.Time //
}
