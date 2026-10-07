// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProvisionOutcomeReceiptDao is the data access object for the table plugin_idc_resource_provision_outcome_receipt.
type ProvisionOutcomeReceiptDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  ProvisionOutcomeReceiptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// ProvisionOutcomeReceiptColumns defines and stores column names for the table plugin_idc_resource_provision_outcome_receipt.
type ProvisionOutcomeReceiptColumns struct {
	Id                      string //
	TenantId                string //
	ReceiptNo               string //
	TaskNo                  string //
	ServiceNo               string //
	StatusCode              string //
	RequestDigest           string //
	TaskEvidenceDigest      string //
	CoreEvidenceDigest      string // Secret-free evidence produced after core commits its service and fulfillment state
	CreatedAt               string //
	OperationNo             string // Frozen opaque fulfillment operation copied from the validated provision request
	CustomerNo              string // Frozen opaque customer owner copied from the validated provision request
	ConfigurationSnapshotNo string // Frozen configuration snapshot copied from the validated provision request
}

// provisionOutcomeReceiptColumns holds the columns for the table plugin_idc_resource_provision_outcome_receipt.
var provisionOutcomeReceiptColumns = ProvisionOutcomeReceiptColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	ReceiptNo:               "receipt_no",
	TaskNo:                  "task_no",
	ServiceNo:               "service_no",
	StatusCode:              "status_code",
	RequestDigest:           "request_digest",
	TaskEvidenceDigest:      "task_evidence_digest",
	CoreEvidenceDigest:      "core_evidence_digest",
	CreatedAt:               "created_at",
	OperationNo:             "operation_no",
	CustomerNo:              "customer_no",
	ConfigurationSnapshotNo: "configuration_snapshot_no",
}

// NewProvisionOutcomeReceiptDao creates and returns a new DAO object for table data access.
func NewProvisionOutcomeReceiptDao(handlers ...gdb.ModelHandler) *ProvisionOutcomeReceiptDao {
	return &ProvisionOutcomeReceiptDao{
		group:    "default",
		table:    "plugin_idc_resource_provision_outcome_receipt",
		columns:  provisionOutcomeReceiptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProvisionOutcomeReceiptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProvisionOutcomeReceiptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProvisionOutcomeReceiptDao) Columns() ProvisionOutcomeReceiptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProvisionOutcomeReceiptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProvisionOutcomeReceiptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProvisionOutcomeReceiptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
