// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateFieldSetDao is the data access object for the table plugin_idc_compliance_contract_template_field_set.
type ContractTemplateFieldSetDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  ContractTemplateFieldSetColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// ContractTemplateFieldSetColumns defines and stores column names for the table plugin_idc_compliance_contract_template_field_set.
type ContractTemplateFieldSetColumns struct {
	Id                       string //
	TenantId                 string //
	FieldSetNo               string //
	TemplateNo               string //
	VersionNo                string //
	ContentRevisionNo        string //
	CommandIdempotencyDigest string //
	CommandFingerprint       string //
	CreatedByUserId          string //
	CreatedAt                string //
}

// contractTemplateFieldSetColumns holds the columns for the table plugin_idc_compliance_contract_template_field_set.
var contractTemplateFieldSetColumns = ContractTemplateFieldSetColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	FieldSetNo:               "field_set_no",
	TemplateNo:               "template_no",
	VersionNo:                "version_no",
	ContentRevisionNo:        "content_revision_no",
	CommandIdempotencyDigest: "command_idempotency_digest",
	CommandFingerprint:       "command_fingerprint",
	CreatedByUserId:          "created_by_user_id",
	CreatedAt:                "created_at",
}

// NewContractTemplateFieldSetDao creates and returns a new DAO object for table data access.
func NewContractTemplateFieldSetDao(handlers ...gdb.ModelHandler) *ContractTemplateFieldSetDao {
	return &ContractTemplateFieldSetDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_template_field_set",
		columns:  contractTemplateFieldSetColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractTemplateFieldSetDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractTemplateFieldSetDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractTemplateFieldSetDao) Columns() ContractTemplateFieldSetColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractTemplateFieldSetDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractTemplateFieldSetDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractTemplateFieldSetDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
