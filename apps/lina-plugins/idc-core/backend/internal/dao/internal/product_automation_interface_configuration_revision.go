// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductAutomationInterfaceConfigurationRevisionDao is the data access object for the table plugin_idc_core_product_automation_interface_configuration_revision.
type ProductAutomationInterfaceConfigurationRevisionDao struct {
	table    string                                                 // table is the underlying table name of the DAO.
	group    string                                                 // group is the database configuration group name of the current DAO.
	columns  ProductAutomationInterfaceConfigurationRevisionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                                     // handlers for customized model modification.
}

// ProductAutomationInterfaceConfigurationRevisionColumns defines and stores column names for the table plugin_idc_core_product_automation_interface_configuration_revision.
type ProductAutomationInterfaceConfigurationRevisionColumns struct {
	Id                                 string //
	TenantId                           string //
	InterfaceInstanceNo                string //
	Revision                           string //
	SchemaVersion                      string //
	DeclarationSnapshot                string //
	PublicValueSnapshot                string //
	DeclarationDigest                  string //
	ValueDigest                        string //
	SecretFieldCount                   string //
	CreatedBy                          string //
	CreatedAt                          string //
	ResourceOwnerConfigurationRevision string // Immutable resource-owner endpoint/account/secret-reference revision; empty for non-resource interfaces / 资源拥有方不可变接口配置修订，非资源接口为空
}

// productAutomationInterfaceConfigurationRevisionColumns holds the columns for the table plugin_idc_core_product_automation_interface_configuration_revision.
var productAutomationInterfaceConfigurationRevisionColumns = ProductAutomationInterfaceConfigurationRevisionColumns{
	Id:                                 "id",
	TenantId:                           "tenant_id",
	InterfaceInstanceNo:                "interface_instance_no",
	Revision:                           "revision",
	SchemaVersion:                      "schema_version",
	DeclarationSnapshot:                "declaration_snapshot",
	PublicValueSnapshot:                "public_value_snapshot",
	DeclarationDigest:                  "declaration_digest",
	ValueDigest:                        "value_digest",
	SecretFieldCount:                   "secret_field_count",
	CreatedBy:                          "created_by",
	CreatedAt:                          "created_at",
	ResourceOwnerConfigurationRevision: "resource_owner_configuration_revision",
}

// NewProductAutomationInterfaceConfigurationRevisionDao creates and returns a new DAO object for table data access.
func NewProductAutomationInterfaceConfigurationRevisionDao(handlers ...gdb.ModelHandler) *ProductAutomationInterfaceConfigurationRevisionDao {
	return &ProductAutomationInterfaceConfigurationRevisionDao{
		group:    "default",
		table:    "plugin_idc_core_product_automation_interface_configuration_revision",
		columns:  productAutomationInterfaceConfigurationRevisionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductAutomationInterfaceConfigurationRevisionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductAutomationInterfaceConfigurationRevisionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductAutomationInterfaceConfigurationRevisionDao) Columns() ProductAutomationInterfaceConfigurationRevisionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductAutomationInterfaceConfigurationRevisionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductAutomationInterfaceConfigurationRevisionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductAutomationInterfaceConfigurationRevisionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
