// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerConsentDao is the data access object for the table plugin_idc_core_customer_consent.
type CustomerConsentDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  CustomerConsentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// CustomerConsentColumns defines and stores column names for the table plugin_idc_core_customer_consent.
type CustomerConsentColumns struct {
	Id              string //
	TenantId        string //
	ConsentNo       string //
	CustomerNo      string //
	ConsentTypeCode string //
	PolicyReference string //
	PolicyVersion   string //
	ActionCode      string //
	SourceCode      string //
	Locale          string //
	EvidenceDigest  string // Digest of canonical consent evidence; raw request metadata remains outside ordinary fields
	OccurredAt      string //
	RecordedBy      string //
	CreatedAt       string //
}

// customerConsentColumns holds the columns for the table plugin_idc_core_customer_consent.
var customerConsentColumns = CustomerConsentColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	ConsentNo:       "consent_no",
	CustomerNo:      "customer_no",
	ConsentTypeCode: "consent_type_code",
	PolicyReference: "policy_reference",
	PolicyVersion:   "policy_version",
	ActionCode:      "action_code",
	SourceCode:      "source_code",
	Locale:          "locale",
	EvidenceDigest:  "evidence_digest",
	OccurredAt:      "occurred_at",
	RecordedBy:      "recorded_by",
	CreatedAt:       "created_at",
}

// NewCustomerConsentDao creates and returns a new DAO object for table data access.
func NewCustomerConsentDao(handlers ...gdb.ModelHandler) *CustomerConsentDao {
	return &CustomerConsentDao{
		group:    "default",
		table:    "plugin_idc_core_customer_consent",
		columns:  customerConsentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerConsentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerConsentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerConsentDao) Columns() CustomerConsentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerConsentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerConsentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerConsentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
