// This file defines the neutral business-audit write seam used inside the core plugin.

package idccap

import "context"

// BusinessAuditTransition contains caller-owned business semantics only. Actor
// identity and request metadata are resolved by the audit owner from trusted context.
type BusinessAuditTransition struct {
	IdempotencyScope   string
	IdempotencyKey     string
	OperationCode      string
	TargetType         string
	TargetNo           string
	CustomerNo         string
	ResultCode         string
	ReasonCode         string
	RiskCode           string
	RequiredPermission string
	CorrelationNo      string
	BeforeSnapshot     any
	AfterSnapshot      any
}

// BusinessAuditBatchTransition records one bounded operation that affects
// several customers. AffectedCustomerNos are indexed through immutable audit
// links; callers must not emit one audit event per customer.
type BusinessAuditBatchTransition struct {
	IdempotencyScope    string
	IdempotencyKey      string
	OperationCode       string
	TargetType          string
	TargetNo            string
	AffectedCustomerNos []string
	ResultCode          string
	ReasonCode          string
	RiskCode            string
	RequiredPermission  string
	CorrelationNo       string
	BeforeSnapshot      any
	AfterSnapshot       any
}

// BusinessAuditAppendResult lets a business owner stop before replaying an
// already committed mutation with the same idempotency semantics.
type BusinessAuditAppendResult struct {
	AuditNo  string
	Replayed bool
}

// BusinessAuditWriter appends one transition inside the caller's active
// authoritative transaction. Implementations must fail the business write when
// audit persistence or integrity sealing fails.
type BusinessAuditWriter interface {
	AppendBusinessTransition(ctx context.Context, input BusinessAuditTransition) error
	AppendBusinessBatchTransition(
		ctx context.Context,
		input BusinessAuditBatchTransition,
	) (BusinessAuditAppendResult, error)
}
