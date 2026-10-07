// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductAutomationInterfaceInstanceDao is the data access object for the table plugin_idc_core_product_automation_interface_instance.
type ProductAutomationInterfaceInstanceDao struct {
	table    string                                    // table is the underlying table name of the DAO.
	group    string                                    // group is the database configuration group name of the current DAO.
	columns  ProductAutomationInterfaceInstanceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                        // handlers for customized model modification.
}

// ProductAutomationInterfaceInstanceColumns defines and stores column names for the table plugin_idc_core_product_automation_interface_instance.
type ProductAutomationInterfaceInstanceColumns struct {
	Id                   string //
	TenantId             string //
	InterfaceInstanceNo  string //
	DisplayName          string //
	PluginId             string //
	ExtensionPointCode   string //
	PluginReleaseVersion string //
	ContractVersion      string //
	AuthorizedMethods    string //
	ReleaseDigest        string //
	DeclarationDigest    string //
	NewBusinessState     string //
	EverReferenced       string //
	Version              string //
	CreatedBy            string //
	CreatedAt            string //
	ChangedBy            string //
	ChangedAt            string //
	InterfaceOriginCode  string //
	ResourceOwnerId      string //
	ResourceChannelRef   string //
}

// productAutomationInterfaceInstanceColumns holds the columns for the table plugin_idc_core_product_automation_interface_instance.
var productAutomationInterfaceInstanceColumns = ProductAutomationInterfaceInstanceColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	InterfaceInstanceNo:  "interface_instance_no",
	DisplayName:          "display_name",
	PluginId:             "plugin_id",
	ExtensionPointCode:   "extension_point_code",
	PluginReleaseVersion: "plugin_release_version",
	ContractVersion:      "contract_version",
	AuthorizedMethods:    "authorized_methods",
	ReleaseDigest:        "release_digest",
	DeclarationDigest:    "declaration_digest",
	NewBusinessState:     "new_business_state",
	EverReferenced:       "ever_referenced",
	Version:              "version",
	CreatedBy:            "created_by",
	CreatedAt:            "created_at",
	ChangedBy:            "changed_by",
	ChangedAt:            "changed_at",
	InterfaceOriginCode:  "interface_origin_code",
	ResourceOwnerId:      "resource_owner_id",
	ResourceChannelRef:   "resource_channel_ref",
}

// NewProductAutomationInterfaceInstanceDao creates and returns a new DAO object for table data access.
func NewProductAutomationInterfaceInstanceDao(handlers ...gdb.ModelHandler) *ProductAutomationInterfaceInstanceDao {
	return &ProductAutomationInterfaceInstanceDao{
		group:    "default",
		table:    "plugin_idc_core_product_automation_interface_instance",
		columns:  productAutomationInterfaceInstanceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductAutomationInterfaceInstanceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductAutomationInterfaceInstanceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductAutomationInterfaceInstanceDao) Columns() ProductAutomationInterfaceInstanceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductAutomationInterfaceInstanceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductAutomationInterfaceInstanceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductAutomationInterfaceInstanceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
