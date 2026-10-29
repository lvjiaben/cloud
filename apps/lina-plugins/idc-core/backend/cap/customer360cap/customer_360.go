// Package customer360cap defines the source-only, cross-plugin customer 360
// summary boundary. Domain owners expose counts only; raw rows never cross a
// plugin boundary.
package customer360cap

import "context"

const (
	ContractVersion  = 1
	MaximumBatchSize = 100
)

// Code is one stable customer 360 summary owned by exactly one domain.
type Code string

const (
	CodeOrders        Code = "orders"
	CodeBills         Code = "bills"
	CodeTransactions  Code = "transactions"
	CodeServers       Code = "servers"
	CodeTaxInvoices   Code = "tax_invoices"
	CodeTickets       Code = "tickets"
	CodeNotifications Code = "notifications"
	CodeAudits        Code = "audits"
)

// Count is one customer-scoped aggregate. Contributors must return exactly
// one row for every requested customer and every code they declare.
type Count struct {
	CustomerNo string
	Code       Code
	Total      int64
}

// Contributor owns one or more fixed summary codes.
type Contributor interface {
	Codes() []Code
	BatchCount(context.Context, []string) ([]Count, error)
}
