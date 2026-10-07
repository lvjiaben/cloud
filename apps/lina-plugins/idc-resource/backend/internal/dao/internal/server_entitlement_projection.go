// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerEntitlementProjectionDao is the data access object for the table plugin_idc_resource_server_entitlement_projection.
type ServerEntitlementProjectionDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  ServerEntitlementProjectionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// ServerEntitlementProjectionColumns defines and stores column names for the table plugin_idc_resource_server_entitlement_projection.
type ServerEntitlementProjectionColumns struct {
	Id                         string //
	TenantId                   string //
	EntitlementNo              string //
	SourceIntentNo             string //
	CustomerNo                 string // Frozen customer owner delivered through the owner command boundary; never derived from JSON or another plugin table
	ProductNo                  string // Opaque generic product reference frozen by the entitlement-created event
	ProductVersionNo           string // Opaque generic product version reference frozen by the entitlement-created event
	ProductNameZhCn            string // Frozen customer-safe Simplified Chinese product label delivered by the entitlement owner event
	ProductNameEnUs            string // Frozen customer-safe English product label delivered by the entitlement owner event
	LocalResourceNo            string // Deterministic generic entitlement number used as the resource-local identity
	MappingNo                  string //
	ProviderTaskNo             string //
	InterfaceInstanceNo        string //
	ProviderPluginId           string //
	PluginReleaseVersion       string //
	FulfillmentContractVersion string //
	ServerContractVersion      string //
	ReleaseDigest              string //
	DeclarationDigest          string //
	Version                    string //
	BoundAt                    string //
	CreatedAt                  string //
	UpdatedAt                  string //
	ActionReference            string // Stable opaque resource-owner action reference; safe for routing but never rendered as customer-visible text
	ServerNo                   string // Stable customer-visible server business number; distinct from the hidden action reference
}

// serverEntitlementProjectionColumns holds the columns for the table plugin_idc_resource_server_entitlement_projection.
var serverEntitlementProjectionColumns = ServerEntitlementProjectionColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	EntitlementNo:              "entitlement_no",
	SourceIntentNo:             "source_intent_no",
	CustomerNo:                 "customer_no",
	ProductNo:                  "product_no",
	ProductVersionNo:           "product_version_no",
	ProductNameZhCn:            "product_name_zh_cn",
	ProductNameEnUs:            "product_name_en_us",
	LocalResourceNo:            "local_resource_no",
	MappingNo:                  "mapping_no",
	ProviderTaskNo:             "provider_task_no",
	InterfaceInstanceNo:        "interface_instance_no",
	ProviderPluginId:           "provider_plugin_id",
	PluginReleaseVersion:       "plugin_release_version",
	FulfillmentContractVersion: "fulfillment_contract_version",
	ServerContractVersion:      "server_contract_version",
	ReleaseDigest:              "release_digest",
	DeclarationDigest:          "declaration_digest",
	Version:                    "version",
	BoundAt:                    "bound_at",
	CreatedAt:                  "created_at",
	UpdatedAt:                  "updated_at",
	ActionReference:            "action_reference",
	ServerNo:                   "server_no",
}

// NewServerEntitlementProjectionDao creates and returns a new DAO object for table data access.
func NewServerEntitlementProjectionDao(handlers ...gdb.ModelHandler) *ServerEntitlementProjectionDao {
	return &ServerEntitlementProjectionDao{
		group:    "default",
		table:    "plugin_idc_resource_server_entitlement_projection",
		columns:  serverEntitlementProjectionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerEntitlementProjectionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerEntitlementProjectionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerEntitlementProjectionDao) Columns() ServerEntitlementProjectionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerEntitlementProjectionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerEntitlementProjectionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerEntitlementProjectionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
