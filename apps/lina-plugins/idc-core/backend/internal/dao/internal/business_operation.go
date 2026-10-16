// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BusinessOperationDao is the data access object for the table plugin_idc_core_business_operation.
type BusinessOperationDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  BusinessOperationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// BusinessOperationColumns defines and stores column names for the table plugin_idc_core_business_operation.
type BusinessOperationColumns struct {
	Id                          string //
	TenantId                    string //
	OperationNo                 string //
	OperationTypeCode           string //
	CustomerNo                  string //
	ServiceNo                   string //
	CurrencyCode                string //
	PriceSnapshotNo             string //
	QuoteNo                     string //
	OrderNo                     string //
	InvoiceNo                   string //
	StatusCode                  string //
	FinancialSettlementRequired string //
	ExecutionModeCode           string //
	ResourceOwnerCode           string //
	ChannelNo                   string //
	PoolNo                      string //
	ProviderTaskNo              string //
	CapabilityEvidenceNo        string //
	CapabilityEvidenceDigest    string //
	IdempotencyDigest           string //
	RequestDigest               string //
	ContentDigest               string //
	ExpectedServiceVersion      string //
	LastErrorCode               string //
	FinanciallySettledAt        string //
	ExecutionStartedAt          string //
	CompletedAt                 string //
	Version                     string //
	CreatedAt                   string //
	UpdatedAt                   string //
	SourceFulfillmentNo         string // Immutable new-purchase server fulfillment identity
	SettlementNo                string // Unified financial settlement authority for a paid new-purchase operation
	SubjectTypeCode             string //
	SubjectNo                   string //
	ExpectedSubjectVersion      string //
	PayerCustomerNo             string //
}

// businessOperationColumns holds the columns for the table plugin_idc_core_business_operation.
var businessOperationColumns = BusinessOperationColumns{
	Id:                          "id",
	TenantId:                    "tenant_id",
	OperationNo:                 "operation_no",
	OperationTypeCode:           "operation_type_code",
	CustomerNo:                  "customer_no",
	ServiceNo:                   "service_no",
	CurrencyCode:                "currency_code",
	PriceSnapshotNo:             "price_snapshot_no",
	QuoteNo:                     "quote_no",
	OrderNo:                     "order_no",
	InvoiceNo:                   "invoice_no",
	StatusCode:                  "status_code",
	FinancialSettlementRequired: "financial_settlement_required",
	ExecutionModeCode:           "execution_mode_code",
	ResourceOwnerCode:           "resource_owner_code",
	ChannelNo:                   "channel_no",
	PoolNo:                      "pool_no",
	ProviderTaskNo:              "provider_task_no",
	CapabilityEvidenceNo:        "capability_evidence_no",
	CapabilityEvidenceDigest:    "capability_evidence_digest",
	IdempotencyDigest:           "idempotency_digest",
	RequestDigest:               "request_digest",
	ContentDigest:               "content_digest",
	ExpectedServiceVersion:      "expected_service_version",
	LastErrorCode:               "last_error_code",
	FinanciallySettledAt:        "financially_settled_at",
	ExecutionStartedAt:          "execution_started_at",
	CompletedAt:                 "completed_at",
	Version:                     "version",
	CreatedAt:                   "created_at",
	UpdatedAt:                   "updated_at",
	SourceFulfillmentNo:         "source_fulfillment_no",
	SettlementNo:                "settlement_no",
	SubjectTypeCode:             "subject_type_code",
	SubjectNo:                   "subject_no",
	ExpectedSubjectVersion:      "expected_subject_version",
	PayerCustomerNo:             "payer_customer_no",
}

// NewBusinessOperationDao creates and returns a new DAO object for table data access.
func NewBusinessOperationDao(handlers ...gdb.ModelHandler) *BusinessOperationDao {
	return &BusinessOperationDao{
		group:    "default",
		table:    "plugin_idc_core_business_operation",
		columns:  businessOperationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BusinessOperationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BusinessOperationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BusinessOperationDao) Columns() BusinessOperationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BusinessOperationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BusinessOperationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BusinessOperationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
