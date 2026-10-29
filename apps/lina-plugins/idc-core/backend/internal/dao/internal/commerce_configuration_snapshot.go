// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceConfigurationSnapshotDao is the data access object for the table plugin_idc_core_commerce_configuration_snapshot.
type CommerceConfigurationSnapshotDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  CommerceConfigurationSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// CommerceConfigurationSnapshotColumns defines and stores column names for the table plugin_idc_core_commerce_configuration_snapshot.
type CommerceConfigurationSnapshotColumns struct {
	Id                      string //
	TenantId                string //
	ConfigurationSnapshotNo string //
	ProductNo               string //
	VersionNo               string //
	SourceRevisionDigest    string //
	SelectionDigest         string //
	SelectedOptionCount     string //
	TotalSelectedQuantity   string //
	ContentDigest           string //
	CreatedAt               string //
	CustomerValueDigest     string //
	TypedValueSnapshot      string // Compiler-produced customer values for native imported pricing; NULL for historical snapshots
	ServerLoginToken        string //
}

// commerceConfigurationSnapshotColumns holds the columns for the table plugin_idc_core_commerce_configuration_snapshot.
var commerceConfigurationSnapshotColumns = CommerceConfigurationSnapshotColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	ConfigurationSnapshotNo: "configuration_snapshot_no",
	ProductNo:               "product_no",
	VersionNo:               "version_no",
	SourceRevisionDigest:    "source_revision_digest",
	SelectionDigest:         "selection_digest",
	SelectedOptionCount:     "selected_option_count",
	TotalSelectedQuantity:   "total_selected_quantity",
	ContentDigest:           "content_digest",
	CreatedAt:               "created_at",
	CustomerValueDigest:     "customer_value_digest",
	TypedValueSnapshot:      "typed_value_snapshot",
	ServerLoginToken:        "server_login_token",
}

// NewCommerceConfigurationSnapshotDao creates and returns a new DAO object for table data access.
func NewCommerceConfigurationSnapshotDao(handlers ...gdb.ModelHandler) *CommerceConfigurationSnapshotDao {
	return &CommerceConfigurationSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_configuration_snapshot",
		columns:  commerceConfigurationSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceConfigurationSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceConfigurationSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceConfigurationSnapshotDao) Columns() CommerceConfigurationSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceConfigurationSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceConfigurationSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceConfigurationSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
