// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutEvidenceDao is the data access object for the table plugin_idc_core_sales_commission_payout_evidence.
type SalesCommissionPayoutEvidenceDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  SalesCommissionPayoutEvidenceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// SalesCommissionPayoutEvidenceColumns defines and stores column names for the table plugin_idc_core_sales_commission_payout_evidence.
type SalesCommissionPayoutEvidenceColumns struct {
	Id                       string //
	TenantId                 string //
	EvidenceNo               string //
	PayoutItemNo             string //
	EvidenceTypeCode         string //
	ProtectedObjectReference string //
	EvidenceDigest           string //
	ProviderReferenceDigest  string //
	ObservedAt               string //
}

// salesCommissionPayoutEvidenceColumns holds the columns for the table plugin_idc_core_sales_commission_payout_evidence.
var salesCommissionPayoutEvidenceColumns = SalesCommissionPayoutEvidenceColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	EvidenceNo:               "evidence_no",
	PayoutItemNo:             "payout_item_no",
	EvidenceTypeCode:         "evidence_type_code",
	ProtectedObjectReference: "protected_object_reference",
	EvidenceDigest:           "evidence_digest",
	ProviderReferenceDigest:  "provider_reference_digest",
	ObservedAt:               "observed_at",
}

// NewSalesCommissionPayoutEvidenceDao creates and returns a new DAO object for table data access.
func NewSalesCommissionPayoutEvidenceDao(handlers ...gdb.ModelHandler) *SalesCommissionPayoutEvidenceDao {
	return &SalesCommissionPayoutEvidenceDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_payout_evidence",
		columns:  salesCommissionPayoutEvidenceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionPayoutEvidenceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionPayoutEvidenceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionPayoutEvidenceDao) Columns() SalesCommissionPayoutEvidenceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionPayoutEvidenceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionPayoutEvidenceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionPayoutEvidenceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
