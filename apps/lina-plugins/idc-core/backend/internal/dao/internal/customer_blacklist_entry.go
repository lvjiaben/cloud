// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerBlacklistEntryDao is the data access object for the table plugin_idc_core_customer_blacklist_entry.
type CustomerBlacklistEntryDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  CustomerBlacklistEntryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// CustomerBlacklistEntryColumns defines and stores column names for the table plugin_idc_core_customer_blacklist_entry.
type CustomerBlacklistEntryColumns struct {
	Id                string //
	TenantId          string //
	EntryNo           string //
	CustomerNo        string //
	ScopeCode         string //
	ReasonCode        string //
	ReasonDetail      string //
	EvidenceReference string //
	RiskLevelCode     string //
	StatusCode        string //
	StartsAt          string //
	ExpiresAt         string //
	CreatedBy         string //
	ApprovedBy        string //
	ApprovedAt        string //
	RevokedBy         string //
	RevokedAt         string //
	CreatedAt         string //
	UpdatedAt         string //
}

// customerBlacklistEntryColumns holds the columns for the table plugin_idc_core_customer_blacklist_entry.
var customerBlacklistEntryColumns = CustomerBlacklistEntryColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	EntryNo:           "entry_no",
	CustomerNo:        "customer_no",
	ScopeCode:         "scope_code",
	ReasonCode:        "reason_code",
	ReasonDetail:      "reason_detail",
	EvidenceReference: "evidence_reference",
	RiskLevelCode:     "risk_level_code",
	StatusCode:        "status_code",
	StartsAt:          "starts_at",
	ExpiresAt:         "expires_at",
	CreatedBy:         "created_by",
	ApprovedBy:        "approved_by",
	ApprovedAt:        "approved_at",
	RevokedBy:         "revoked_by",
	RevokedAt:         "revoked_at",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewCustomerBlacklistEntryDao creates and returns a new DAO object for table data access.
func NewCustomerBlacklistEntryDao(handlers ...gdb.ModelHandler) *CustomerBlacklistEntryDao {
	return &CustomerBlacklistEntryDao{
		group:    "default",
		table:    "plugin_idc_core_customer_blacklist_entry",
		columns:  customerBlacklistEntryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerBlacklistEntryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerBlacklistEntryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerBlacklistEntryDao) Columns() CustomerBlacklistEntryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerBlacklistEntryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerBlacklistEntryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerBlacklistEntryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
