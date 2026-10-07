// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditAccountMovementDao is the data access object for the table plugin_idc_core_credit_account_movement.
type CreditAccountMovementDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  CreditAccountMovementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// CreditAccountMovementColumns defines and stores column names for the table plugin_idc_core_credit_account_movement.
type CreditAccountMovementColumns struct {
	Id                    string //
	TenantId              string //
	MovementNo            string //
	CreditAccountNo       string //
	CustomerNo            string //
	CurrencyCode          string //
	ExposureNo            string //
	StatementNo           string //
	RepaymentNo           string //
	MovementTypeCode      string //
	AmountMinor           string //
	AccountVersionBefore  string //
	AccountVersionAfter   string //
	StatusBeforeCode      string //
	StatusAfterCode       string //
	TotalLimitBeforeMinor string //
	TotalLimitAfterMinor  string //
	AvailableBeforeMinor  string //
	AvailableAfterMinor   string //
	UnbilledBeforeMinor   string //
	UnbilledAfterMinor    string //
	BilledBeforeMinor     string //
	BilledAfterMinor      string //
	DelinquentBeforeMinor string //
	DelinquentAfterMinor  string //
	WrittenOffBeforeMinor string //
	WrittenOffAfterMinor  string //
	SourceTypeCode        string //
	SourceReference       string //
	IdempotencyDigest     string //
	EvidenceDigest        string //
	ContentDigest         string //
	ActorTypeCode         string //
	ActorId               string //
	OccurredAt            string //
	CreatedAt             string //
}

// creditAccountMovementColumns holds the columns for the table plugin_idc_core_credit_account_movement.
var creditAccountMovementColumns = CreditAccountMovementColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	MovementNo:            "movement_no",
	CreditAccountNo:       "credit_account_no",
	CustomerNo:            "customer_no",
	CurrencyCode:          "currency_code",
	ExposureNo:            "exposure_no",
	StatementNo:           "statement_no",
	RepaymentNo:           "repayment_no",
	MovementTypeCode:      "movement_type_code",
	AmountMinor:           "amount_minor",
	AccountVersionBefore:  "account_version_before",
	AccountVersionAfter:   "account_version_after",
	StatusBeforeCode:      "status_before_code",
	StatusAfterCode:       "status_after_code",
	TotalLimitBeforeMinor: "total_limit_before_minor",
	TotalLimitAfterMinor:  "total_limit_after_minor",
	AvailableBeforeMinor:  "available_before_minor",
	AvailableAfterMinor:   "available_after_minor",
	UnbilledBeforeMinor:   "unbilled_before_minor",
	UnbilledAfterMinor:    "unbilled_after_minor",
	BilledBeforeMinor:     "billed_before_minor",
	BilledAfterMinor:      "billed_after_minor",
	DelinquentBeforeMinor: "delinquent_before_minor",
	DelinquentAfterMinor:  "delinquent_after_minor",
	WrittenOffBeforeMinor: "written_off_before_minor",
	WrittenOffAfterMinor:  "written_off_after_minor",
	SourceTypeCode:        "source_type_code",
	SourceReference:       "source_reference",
	IdempotencyDigest:     "idempotency_digest",
	EvidenceDigest:        "evidence_digest",
	ContentDigest:         "content_digest",
	ActorTypeCode:         "actor_type_code",
	ActorId:               "actor_id",
	OccurredAt:            "occurred_at",
	CreatedAt:             "created_at",
}

// NewCreditAccountMovementDao creates and returns a new DAO object for table data access.
func NewCreditAccountMovementDao(handlers ...gdb.ModelHandler) *CreditAccountMovementDao {
	return &CreditAccountMovementDao{
		group:    "default",
		table:    "plugin_idc_core_credit_account_movement",
		columns:  creditAccountMovementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditAccountMovementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditAccountMovementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditAccountMovementDao) Columns() CreditAccountMovementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditAccountMovementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditAccountMovementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditAccountMovementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
