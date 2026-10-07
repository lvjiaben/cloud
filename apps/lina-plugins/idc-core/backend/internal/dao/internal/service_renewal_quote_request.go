// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceRenewalQuoteRequestDao is the data access object for the table plugin_idc_core_service_renewal_quote_request.
type ServiceRenewalQuoteRequestDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  ServiceRenewalQuoteRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// ServiceRenewalQuoteRequestColumns defines and stores column names for the table plugin_idc_core_service_renewal_quote_request.
type ServiceRenewalQuoteRequestColumns struct {
	Id                         string //
	TenantId                   string //
	RequestNo                  string //
	CustomerNo                 string //
	EntitlementNo              string //
	CurrencyCode               string //
	RequestSourceCode          string //
	RenewalCycles              string //
	IdempotencyDigest          string //
	RequestDigest              string //
	QuoteNo                    string //
	PriceSnapshotNo            string //
	ExpectedEntitlementVersion string //
	ContentDigest              string //
	CreatedAt                  string //
}

// serviceRenewalQuoteRequestColumns holds the columns for the table plugin_idc_core_service_renewal_quote_request.
var serviceRenewalQuoteRequestColumns = ServiceRenewalQuoteRequestColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	RequestNo:                  "request_no",
	CustomerNo:                 "customer_no",
	EntitlementNo:              "entitlement_no",
	CurrencyCode:               "currency_code",
	RequestSourceCode:          "request_source_code",
	RenewalCycles:              "renewal_cycles",
	IdempotencyDigest:          "idempotency_digest",
	RequestDigest:              "request_digest",
	QuoteNo:                    "quote_no",
	PriceSnapshotNo:            "price_snapshot_no",
	ExpectedEntitlementVersion: "expected_entitlement_version",
	ContentDigest:              "content_digest",
	CreatedAt:                  "created_at",
}

// NewServiceRenewalQuoteRequestDao creates and returns a new DAO object for table data access.
func NewServiceRenewalQuoteRequestDao(handlers ...gdb.ModelHandler) *ServiceRenewalQuoteRequestDao {
	return &ServiceRenewalQuoteRequestDao{
		group:    "default",
		table:    "plugin_idc_core_service_renewal_quote_request",
		columns:  serviceRenewalQuoteRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceRenewalQuoteRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceRenewalQuoteRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceRenewalQuoteRequestDao) Columns() ServiceRenewalQuoteRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceRenewalQuoteRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceRenewalQuoteRequestDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *ServiceRenewalQuoteRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
