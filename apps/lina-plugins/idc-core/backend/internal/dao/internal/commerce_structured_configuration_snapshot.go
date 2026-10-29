// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceStructuredConfigurationSnapshotDao is the data access object for the table plugin_idc_core_commerce_structured_configuration_snapshot.
type CommerceStructuredConfigurationSnapshotDao struct {
	table    string                                         // table is the underlying table name of the DAO.
	group    string                                         // group is the database configuration group name of the current DAO.
	columns  CommerceStructuredConfigurationSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                             // handlers for customized model modification.
}

// CommerceStructuredConfigurationSnapshotColumns defines and stores column names for the table plugin_idc_core_commerce_structured_configuration_snapshot.
type CommerceStructuredConfigurationSnapshotColumns struct {
	Id                             string //
	TenantId                       string //
	SnapshotNo                     string //
	CustomerNo                     string //
	UploaderUserId                 string //
	QuoteNo                        string //
	QuoteItemNo                    string //
	ProductNo                      string //
	VersionNo                      string //
	ConfigurationRevision          string //
	InterfaceInstanceNo            string //
	InterfaceConfigurationRevision string //
	CustomerDeclarationDigest      string //
	CandidateSnapshotDigest        string //
	CustomerValueDigest            string //
	CandidateSnapshot              string //
	TypedValueSnapshot             string //
	ContentDigest                  string //
	OrderNo                        string //
	OrderItemNo                    string //
	BoundAt                        string //
	CreatedAt                      string //
	ServerLoginToken               string //
}

// commerceStructuredConfigurationSnapshotColumns holds the columns for the table plugin_idc_core_commerce_structured_configuration_snapshot.
var commerceStructuredConfigurationSnapshotColumns = CommerceStructuredConfigurationSnapshotColumns{
	Id:                             "id",
	TenantId:                       "tenant_id",
	SnapshotNo:                     "snapshot_no",
	CustomerNo:                     "customer_no",
	UploaderUserId:                 "uploader_user_id",
	QuoteNo:                        "quote_no",
	QuoteItemNo:                    "quote_item_no",
	ProductNo:                      "product_no",
	VersionNo:                      "version_no",
	ConfigurationRevision:          "configuration_revision",
	InterfaceInstanceNo:            "interface_instance_no",
	InterfaceConfigurationRevision: "interface_configuration_revision",
	CustomerDeclarationDigest:      "customer_declaration_digest",
	CandidateSnapshotDigest:        "candidate_snapshot_digest",
	CustomerValueDigest:            "customer_value_digest",
	CandidateSnapshot:              "candidate_snapshot",
	TypedValueSnapshot:             "typed_value_snapshot",
	ContentDigest:                  "content_digest",
	OrderNo:                        "order_no",
	OrderItemNo:                    "order_item_no",
	BoundAt:                        "bound_at",
	CreatedAt:                      "created_at",
	ServerLoginToken:               "server_login_token",
}

// NewCommerceStructuredConfigurationSnapshotDao creates and returns a new DAO object for table data access.
func NewCommerceStructuredConfigurationSnapshotDao(handlers ...gdb.ModelHandler) *CommerceStructuredConfigurationSnapshotDao {
	return &CommerceStructuredConfigurationSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_structured_configuration_snapshot",
		columns:  commerceStructuredConfigurationSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceStructuredConfigurationSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceStructuredConfigurationSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceStructuredConfigurationSnapshotDao) Columns() CommerceStructuredConfigurationSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceStructuredConfigurationSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceStructuredConfigurationSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceStructuredConfigurationSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
