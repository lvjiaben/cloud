// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelDao is the data access object for the table plugin_idc_payment_channel.
type ChannelDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ChannelColumns     // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ChannelColumns defines and stores column names for the table plugin_idc_payment_channel.
type ChannelColumns struct {
	Id                  string //
	TenantId            string //
	ChannelNo           string //
	ProviderCode        string //
	DisplayName         string //
	EndpointOrigin      string //
	MerchantReference   string //
	SecretRef           string //
	SupportedModes      string //
	SupportedCurrencies string //
	CapabilitySnapshot  string //
	StatusCode          string //
	VerifiedAt          string //
	Version             string //
	CreatedBy           string //
	UpdatedBy           string //
	CreatedAt           string //
	UpdatedAt           string //
	DeletedAt           string //
}

// channelColumns holds the columns for the table plugin_idc_payment_channel.
var channelColumns = ChannelColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	ChannelNo:           "channel_no",
	ProviderCode:        "provider_code",
	DisplayName:         "display_name",
	EndpointOrigin:      "endpoint_origin",
	MerchantReference:   "merchant_reference",
	SecretRef:           "secret_ref",
	SupportedModes:      "supported_modes",
	SupportedCurrencies: "supported_currencies",
	CapabilitySnapshot:  "capability_snapshot",
	StatusCode:          "status_code",
	VerifiedAt:          "verified_at",
	Version:             "version",
	CreatedBy:           "created_by",
	UpdatedBy:           "updated_by",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
	DeletedAt:           "deleted_at",
}

// NewChannelDao creates and returns a new DAO object for table data access.
func NewChannelDao(handlers ...gdb.ModelHandler) *ChannelDao {
	return &ChannelDao{
		group:    "default",
		table:    "plugin_idc_payment_channel",
		columns:  channelColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelDao) Columns() ChannelColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
