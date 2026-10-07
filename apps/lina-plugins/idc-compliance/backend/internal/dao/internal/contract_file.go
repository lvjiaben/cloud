// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractFileDao is the data access object for the table plugin_idc_compliance_contract_file.
type ContractFileDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  ContractFileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// ContractFileColumns defines and stores column names for the table plugin_idc_compliance_contract_file.
type ContractFileColumns struct {
	Id                 string //
	TenantId           string //
	FileNo             string //
	ContractNo         string //
	FlowNo             string //
	FileTypeCode       string //
	PrivateObjectRef   string //
	ContentSha256      string //
	MediaType          string //
	ByteSize           string //
	StatusCode         string //
	Immutable          string //
	ArchivedAt         string //
	RetentionUntil     string //
	LegalHold          string //
	CreatedAt          string //
	UpdatedAt          string //
	SupplierObservedAt string //
}

// contractFileColumns holds the columns for the table plugin_idc_compliance_contract_file.
var contractFileColumns = ContractFileColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	FileNo:             "file_no",
	ContractNo:         "contract_no",
	FlowNo:             "flow_no",
	FileTypeCode:       "file_type_code",
	PrivateObjectRef:   "private_object_ref",
	ContentSha256:      "content_sha256",
	MediaType:          "media_type",
	ByteSize:           "byte_size",
	StatusCode:         "status_code",
	Immutable:          "immutable",
	ArchivedAt:         "archived_at",
	RetentionUntil:     "retention_until",
	LegalHold:          "legal_hold",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
	SupplierObservedAt: "supplier_observed_at",
}

// NewContractFileDao creates and returns a new DAO object for table data access.
func NewContractFileDao(handlers ...gdb.ModelHandler) *ContractFileDao {
	return &ContractFileDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_file",
		columns:  contractFileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractFileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractFileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractFileDao) Columns() ContractFileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractFileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractFileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractFileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
