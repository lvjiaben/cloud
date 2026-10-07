// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceFulfillmentUnitDao is the data access object for the table plugin_idc_core_commerce_fulfillment_unit.
type CommerceFulfillmentUnitDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  CommerceFulfillmentUnitColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// CommerceFulfillmentUnitColumns defines and stores column names for the table plugin_idc_core_commerce_fulfillment_unit.
type CommerceFulfillmentUnitColumns struct {
	Id                  string //
	TenantId            string //
	FulfillmentNo       string //
	OrderNo             string //
	OrderItemNo         string //
	UnitSequence        string //
	StatusCode          string //
	ReservationNo       string //
	BusinessOperationNo string //
	ServiceNo           string //
	LastEvidenceDigest  string //
	StartedAt           string //
	CompletedAt         string //
	Version             string //
	CreatedAt           string //
	UpdatedAt           string //
}

// commerceFulfillmentUnitColumns holds the columns for the table plugin_idc_core_commerce_fulfillment_unit.
var commerceFulfillmentUnitColumns = CommerceFulfillmentUnitColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	FulfillmentNo:       "fulfillment_no",
	OrderNo:             "order_no",
	OrderItemNo:         "order_item_no",
	UnitSequence:        "unit_sequence",
	StatusCode:          "status_code",
	ReservationNo:       "reservation_no",
	BusinessOperationNo: "business_operation_no",
	ServiceNo:           "service_no",
	LastEvidenceDigest:  "last_evidence_digest",
	StartedAt:           "started_at",
	CompletedAt:         "completed_at",
	Version:             "version",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewCommerceFulfillmentUnitDao creates and returns a new DAO object for table data access.
func NewCommerceFulfillmentUnitDao(handlers ...gdb.ModelHandler) *CommerceFulfillmentUnitDao {
	return &CommerceFulfillmentUnitDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_fulfillment_unit",
		columns:  commerceFulfillmentUnitColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceFulfillmentUnitDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceFulfillmentUnitDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceFulfillmentUnitDao) Columns() CommerceFulfillmentUnitColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceFulfillmentUnitDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceFulfillmentUnitDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceFulfillmentUnitDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
