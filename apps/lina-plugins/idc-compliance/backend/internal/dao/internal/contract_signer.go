// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractSignerDao is the data access object for the table plugin_idc_compliance_contract_signer.
type ContractSignerDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  ContractSignerColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// ContractSignerColumns defines and stores column names for the table plugin_idc_compliance_contract_signer.
type ContractSignerColumns struct {
	Id               string //
	TenantId         string //
	SignerNo         string //
	FlowNo           string //
	SequenceNo       string //
	SubjectTypeCode  string //
	SubjectNo        string //
	RoleCode         string //
	ProviderActorRef string //
	SnapshotSha256   string //
	StatusCode       string //
	SignedAt         string //
	CreatedAt        string //
	UpdatedAt        string //
}

// contractSignerColumns holds the columns for the table plugin_idc_compliance_contract_signer.
var contractSignerColumns = ContractSignerColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	SignerNo:         "signer_no",
	FlowNo:           "flow_no",
	SequenceNo:       "sequence_no",
	SubjectTypeCode:  "subject_type_code",
	SubjectNo:        "subject_no",
	RoleCode:         "role_code",
	ProviderActorRef: "provider_actor_ref",
	SnapshotSha256:   "snapshot_sha256",
	StatusCode:       "status_code",
	SignedAt:         "signed_at",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewContractSignerDao creates and returns a new DAO object for table data access.
func NewContractSignerDao(handlers ...gdb.ModelHandler) *ContractSignerDao {
	return &ContractSignerDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_signer",
		columns:  contractSignerColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractSignerDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractSignerDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractSignerDao) Columns() ContractSignerColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractSignerDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractSignerDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractSignerDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
