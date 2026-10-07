// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryQuoteDao is the data access object for the table plugin_idc_core_server_accessory_quote.
type ServerAccessoryQuoteDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ServerAccessoryQuoteColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ServerAccessoryQuoteColumns defines and stores column names for the table plugin_idc_core_server_accessory_quote.
type ServerAccessoryQuoteColumns struct {
	Id                     string //
	TenantId               string //
	QuoteNo                string //
	CustomerNo             string //
	ServiceNo              string //
	ActionCode             string //
	AccessoryKind          string //
	CurrencyCode           string //
	RequestedQuantity      string //
	ExistingObjectCount    string //
	ReservedCreateCount    string //
	FreeCountPerServer     string //
	ChargedQuantity        string //
	UnitAmountMinor        string //
	TotalAmountMinor       string //
	PeriodUnitCode         string //
	PeriodCount            string //
	PolicyNo               string //
	PolicyVersion          string //
	IdempotencyDigest      string //
	RequestDigest          string //
	ContentDigest          string //
	IssuedAt               string //
	ExpiresAt              string //
	Parameters             string // Canonical provider parameters frozen by the customer-visible quote
	CustomerSubjectVersion string //
	ExpectedServiceVersion string //
	SubtotalMinor          string //
	NetAmountMinor         string //
	TaxAmountMinor         string //
	TaxPolicyNo            string //
	TaxTreatmentCode       string //
	TaxRateBasisPoints     string //
	TaxCategoryCode        string //
	RoundingScopeCode      string //
	RoundingModeCode       string //
	RoundingQuantumMinor   string //
	BaselineObjectNos      string //
	BaselineSnapshotNo     string //
	BaselineObservedAt     string //
}

// serverAccessoryQuoteColumns holds the columns for the table plugin_idc_core_server_accessory_quote.
var serverAccessoryQuoteColumns = ServerAccessoryQuoteColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	QuoteNo:                "quote_no",
	CustomerNo:             "customer_no",
	ServiceNo:              "service_no",
	ActionCode:             "action_code",
	AccessoryKind:          "accessory_kind",
	CurrencyCode:           "currency_code",
	RequestedQuantity:      "requested_quantity",
	ExistingObjectCount:    "existing_object_count",
	ReservedCreateCount:    "reserved_create_count",
	FreeCountPerServer:     "free_count_per_server",
	ChargedQuantity:        "charged_quantity",
	UnitAmountMinor:        "unit_amount_minor",
	TotalAmountMinor:       "total_amount_minor",
	PeriodUnitCode:         "period_unit_code",
	PeriodCount:            "period_count",
	PolicyNo:               "policy_no",
	PolicyVersion:          "policy_version",
	IdempotencyDigest:      "idempotency_digest",
	RequestDigest:          "request_digest",
	ContentDigest:          "content_digest",
	IssuedAt:               "issued_at",
	ExpiresAt:              "expires_at",
	Parameters:             "parameters",
	CustomerSubjectVersion: "customer_subject_version",
	ExpectedServiceVersion: "expected_service_version",
	SubtotalMinor:          "subtotal_minor",
	NetAmountMinor:         "net_amount_minor",
	TaxAmountMinor:         "tax_amount_minor",
	TaxPolicyNo:            "tax_policy_no",
	TaxTreatmentCode:       "tax_treatment_code",
	TaxRateBasisPoints:     "tax_rate_basis_points",
	TaxCategoryCode:        "tax_category_code",
	RoundingScopeCode:      "rounding_scope_code",
	RoundingModeCode:       "rounding_mode_code",
	RoundingQuantumMinor:   "rounding_quantum_minor",
	BaselineObjectNos:      "baseline_object_nos",
	BaselineSnapshotNo:     "baseline_snapshot_no",
	BaselineObservedAt:     "baseline_observed_at",
}

// NewServerAccessoryQuoteDao creates and returns a new DAO object for table data access.
func NewServerAccessoryQuoteDao(handlers ...gdb.ModelHandler) *ServerAccessoryQuoteDao {
	return &ServerAccessoryQuoteDao{
		group:    "default",
		table:    "plugin_idc_core_server_accessory_quote",
		columns:  serverAccessoryQuoteColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerAccessoryQuoteDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerAccessoryQuoteDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerAccessoryQuoteDao) Columns() ServerAccessoryQuoteColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerAccessoryQuoteDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerAccessoryQuoteDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerAccessoryQuoteDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
