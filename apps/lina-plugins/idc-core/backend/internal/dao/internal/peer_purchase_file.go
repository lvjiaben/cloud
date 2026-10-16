// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PeerPurchaseFileDao is the data access object for the table plugin_idc_core_peer_purchase_file.
type PeerPurchaseFileDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  PeerPurchaseFileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// PeerPurchaseFileColumns defines and stores column names for the table plugin_idc_core_peer_purchase_file.
type PeerPurchaseFileColumns struct {
	Id                string //
	TenantId          string //
	IntentNo          string //
	FieldKey          string //
	FileSequence      string //
	Sha256Hex         string //
	UpstreamFileToken string //
	CreatedAt         string //
}

// peerPurchaseFileColumns holds the columns for the table plugin_idc_core_peer_purchase_file.
var peerPurchaseFileColumns = PeerPurchaseFileColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	IntentNo:          "intent_no",
	FieldKey:          "field_key",
	FileSequence:      "file_sequence",
	Sha256Hex:         "sha256_hex",
	UpstreamFileToken: "upstream_file_token",
	CreatedAt:         "created_at",
}

// NewPeerPurchaseFileDao creates and returns a new DAO object for table data access.
func NewPeerPurchaseFileDao(handlers ...gdb.ModelHandler) *PeerPurchaseFileDao {
	return &PeerPurchaseFileDao{
		group:    "default",
		table:    "plugin_idc_core_peer_purchase_file",
		columns:  peerPurchaseFileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PeerPurchaseFileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PeerPurchaseFileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PeerPurchaseFileDao) Columns() PeerPurchaseFileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PeerPurchaseFileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PeerPurchaseFileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PeerPurchaseFileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
