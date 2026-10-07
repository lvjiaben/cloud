// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MappingDao is the data access object for the table plugin_idc_resource_mapping.
type MappingDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  MappingColumns     // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// MappingColumns defines and stores column names for the table plugin_idc_resource_mapping.
type MappingColumns struct {
	Id                  string //
	TenantId            string //
	MappingNo           string //
	LocalResourceNo     string // Opaque core-owned resource number; no cross-plugin database foreign key is used
	ChannelNo           string //
	PoolNo              string //
	ExternalResourceRef string // Opaque Provider resource reference, never interpreted as a local identity
	StatusCode          string //
	BoundAt             string //
	ReleasedAt          string //
	Version             string //
	CreatedAt           string //
	UpdatedAt           string //
}

// mappingColumns holds the columns for the table plugin_idc_resource_mapping.
var mappingColumns = MappingColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	MappingNo:           "mapping_no",
	LocalResourceNo:     "local_resource_no",
	ChannelNo:           "channel_no",
	PoolNo:              "pool_no",
	ExternalResourceRef: "external_resource_ref",
	StatusCode:          "status_code",
	BoundAt:             "bound_at",
	ReleasedAt:          "released_at",
	Version:             "version",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewMappingDao creates and returns a new DAO object for table data access.
func NewMappingDao(handlers ...gdb.ModelHandler) *MappingDao {
	return &MappingDao{
		group:    "default",
		table:    "plugin_idc_resource_mapping",
		columns:  mappingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MappingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MappingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MappingDao) Columns() MappingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MappingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MappingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MappingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
