// Package customerclosurecap defines the source-only, cross-plugin closure
// precheck boundary. Each owner reports only bounded aggregate counts for the
// customer records it owns; raw rows never cross the boundary.
package customerclosurecap

import (
	"context"
	"time"
)

const (
	ContractVersion  = 1
	MaximumBatchSize = 100
)

// Code is one stable closure-precheck finding owned by a business domain.
type Code string

const (
	CodeUnsettledFunds       Code = "unsettled_funds"
	CodeActiveServices       Code = "active_services"
	CodeUnsettledInvoices    Code = "unsettled_invoices"
	CodeInProgressOperations Code = "in_progress_operations"
	CodeLegalRetention       Code = "legal_retention"
)

// Count is one customer-scoped aggregate. Zero values are required so the
// coordinator can distinguish a complete answer from a missing owner.
type Count struct {
	CustomerNo string
	Code       Code
	Count      int
}

// Contributor owns one or more fixed finding codes and returns exactly one
// count for every requested customer and declared code.
type Contributor interface {
	Codes() []Code
	BatchCount(context.Context, []string, time.Time) ([]Count, error)
}
