// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrincipalMappingDao is the data access object for the table plugin_idc_resource_principal_mapping.
type PrincipalMappingDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  PrincipalMappingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// PrincipalMappingColumns defines and stores column names for the table plugin_idc_resource_principal_mapping.
type PrincipalMappingColumns struct {
	Id                   string //
	TenantId             string //
	MappingNo            string //
	CustomerNo           string // Opaque core-owned customer reference with no cross-plugin database foreign key
	ChannelNo            string //
	ExternalPrincipalRef string // Channel-local upstream client identifier; never inferred from the local customer number
	StatusCode           string //
	EvidenceDigest       string // Secret-free digest of the authoritative mapping evidence
	ObservedAt           string //
	Version              string //
	CreatedAt            string //
	UpdatedAt            string //
}

// principalMappingColumns holds the columns for the table plugin_idc_resource_principal_mapping.
var principalMappingColumns = PrincipalMappingColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	MappingNo:            "mapping_no",
	CustomerNo:           "customer_no",
	ChannelNo:            "channel_no",
	ExternalPrincipalRef: "external_principal_ref",
	StatusCode:           "status_code",
	EvidenceDigest:       "evidence_digest",
	ObservedAt:           "observed_at",
	Version:              "version",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewPrincipalMappingDao creates and returns a new DAO object for table data access.
func NewPrincipalMappingDao(handlers ...gdb.ModelHandler) *PrincipalMappingDao {
	return &PrincipalMappingDao{
		group:    "default",
		table:    "plugin_idc_resource_principal_mapping",
		columns:  principalMappingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrincipalMappingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrincipalMappingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrincipalMappingDao) Columns() PrincipalMappingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrincipalMappingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrincipalMappingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrincipalMappingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
