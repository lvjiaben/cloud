// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractAutoSignUsageDao is the data access object for the table plugin_idc_compliance_contract_auto_sign_usage.
type ContractAutoSignUsageDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  ContractAutoSignUsageColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// ContractAutoSignUsageColumns defines and stores column names for the table plugin_idc_compliance_contract_auto_sign_usage.
type ContractAutoSignUsageColumns struct {
	Id                       string //
	TenantId                 string //
	UsageNo                  string //
	AuthorizationNo          string //
	ContractNo               string //
	CurrencyCode             string //
	AmountMinor              string //
	CommandIdempotencyDigest string //
	CommandFingerprint       string //
	ProviderAcceptedAt       string //
	CreatedAt                string //
}

// contractAutoSignUsageColumns holds the columns for the table plugin_idc_compliance_contract_auto_sign_usage.
var contractAutoSignUsageColumns = ContractAutoSignUsageColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	UsageNo:                  "usage_no",
	AuthorizationNo:          "authorization_no",
	ContractNo:               "contract_no",
	CurrencyCode:             "currency_code",
	AmountMinor:              "amount_minor",
	CommandIdempotencyDigest: "command_idempotency_digest",
	CommandFingerprint:       "command_fingerprint",
	ProviderAcceptedAt:       "provider_accepted_at",
	CreatedAt:                "created_at",
}

// NewContractAutoSignUsageDao creates and returns a new DAO object for table data access.
func NewContractAutoSignUsageDao(handlers ...gdb.ModelHandler) *ContractAutoSignUsageDao {
	return &ContractAutoSignUsageDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_auto_sign_usage",
		columns:  contractAutoSignUsageColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractAutoSignUsageDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractAutoSignUsageDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractAutoSignUsageDao) Columns() ContractAutoSignUsageColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractAutoSignUsageDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractAutoSignUsageDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractAutoSignUsageDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
