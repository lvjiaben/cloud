// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSettlementDeliveryDao is the data access object for the table plugin_idc_core_financial_settlement_delivery.
type FinancialSettlementDeliveryDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  FinancialSettlementDeliveryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// FinancialSettlementDeliveryColumns defines and stores column names for the table plugin_idc_core_financial_settlement_delivery.
type FinancialSettlementDeliveryColumns struct {
	Id                string //
	TenantId          string //
	DeliveryNo        string //
	EventNo           string //
	EventType         string //
	SettlementNo      string //
	OrderNo           string //
	InvoiceNo         string //
	CustomerNo        string //
	CurrencyCode      string //
	FulfillmentCount  string //
	ReadyCount        string //
	WaitingGateCount  string //
	ManualReviewCount string //
	EvidenceDigest    string //
	ContentDigest     string //
	CreatedAt         string //
}

// financialSettlementDeliveryColumns holds the columns for the table plugin_idc_core_financial_settlement_delivery.
var financialSettlementDeliveryColumns = FinancialSettlementDeliveryColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	DeliveryNo:        "delivery_no",
	EventNo:           "event_no",
	EventType:         "event_type",
	SettlementNo:      "settlement_no",
	OrderNo:           "order_no",
	InvoiceNo:         "invoice_no",
	CustomerNo:        "customer_no",
	CurrencyCode:      "currency_code",
	FulfillmentCount:  "fulfillment_count",
	ReadyCount:        "ready_count",
	WaitingGateCount:  "waiting_gate_count",
	ManualReviewCount: "manual_review_count",
	EvidenceDigest:    "evidence_digest",
	ContentDigest:     "content_digest",
	CreatedAt:         "created_at",
}

// NewFinancialSettlementDeliveryDao creates and returns a new DAO object for table data access.
func NewFinancialSettlementDeliveryDao(handlers ...gdb.ModelHandler) *FinancialSettlementDeliveryDao {
	return &FinancialSettlementDeliveryDao{
		group:    "default",
		table:    "plugin_idc_core_financial_settlement_delivery",
		columns:  financialSettlementDeliveryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialSettlementDeliveryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialSettlementDeliveryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialSettlementDeliveryDao) Columns() FinancialSettlementDeliveryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialSettlementDeliveryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialSettlementDeliveryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialSettlementDeliveryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
