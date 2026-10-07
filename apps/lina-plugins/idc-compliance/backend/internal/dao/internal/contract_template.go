// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateDao is the data access object for the table plugin_idc_compliance_contract_template.
type ContractTemplateDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  ContractTemplateColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// ContractTemplateColumns defines and stores column names for the table plugin_idc_compliance_contract_template.
type ContractTemplateColumns struct {
	Id                        string //
	TenantId                  string //
	TemplateNo                string //
	OriginCode                string //
	ProviderCode              string //
	ProviderTemplateRef       string // Opaque Tencent template identifier; it is never accepted from browser create or update commands
	ProviderStateCode         string //
	ProviderLastSeenSyncNo    string //
	ProviderUpdatedAt         string //
	CreationIdempotencyDigest string //
	CreationFingerprint       string //
	CreatedAt                 string //
}

// contractTemplateColumns holds the columns for the table plugin_idc_compliance_contract_template.
var contractTemplateColumns = ContractTemplateColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	TemplateNo:                "template_no",
	OriginCode:                "origin_code",
	ProviderCode:              "provider_code",
	ProviderTemplateRef:       "provider_template_ref",
	ProviderStateCode:         "provider_state_code",
	ProviderLastSeenSyncNo:    "provider_last_seen_sync_no",
	ProviderUpdatedAt:         "provider_updated_at",
	CreationIdempotencyDigest: "creation_idempotency_digest",
	CreationFingerprint:       "creation_fingerprint",
	CreatedAt:                 "created_at",
}

// NewContractTemplateDao creates and returns a new DAO object for table data access.
func NewContractTemplateDao(handlers ...gdb.ModelHandler) *ContractTemplateDao {
	return &ContractTemplateDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_template",
		columns:  contractTemplateColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractTemplateDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractTemplateDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractTemplateDao) Columns() ContractTemplateColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractTemplateDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractTemplateDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractTemplateDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
