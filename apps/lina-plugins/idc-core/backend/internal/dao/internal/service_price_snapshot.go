// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServicePriceSnapshotDao is the data access object for the table plugin_idc_core_service_price_snapshot.
type ServicePriceSnapshotDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ServicePriceSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ServicePriceSnapshotColumns defines and stores column names for the table plugin_idc_core_service_price_snapshot.
type ServicePriceSnapshotColumns struct {
	Id                                string //
	TenantId                          string //
	PriceSnapshotNo                   string //
	ServiceNo                         string //
	CustomerNo                        string //
	PhaseCode                         string //
	SourceQuoteNo                     string //
	SourceQuoteItemNo                 string //
	ProductNo                         string //
	VersionNo                         string //
	BillingRuleNo                     string //
	ConfigurationSnapshotNo           string //
	CurrencyCode                      string //
	PurchaseCycles                    string //
	PeriodUnitCode                    string //
	PeriodCount                       string //
	RenewalModeCode                   string //
	CapabilityEvidenceNo              string //
	CapabilityEvidenceDigest          string //
	CapabilityEvidenceObservedAt      string //
	CapabilityEvidenceExpiresAt       string //
	OriginalExpiresAt                 string //
	ProjectedExpiresAt                string //
	TaxPolicyNo                       string //
	PriceIncludesTax                  string //
	TaxTreatmentCode                  string //
	TaxRateBasisPoints                string //
	TaxCategoryCode                   string //
	RoundingScopeCode                 string //
	RoundingModeCode                  string //
	RoundingQuantumMinor              string //
	ComponentCount                    string //
	SubtotalMinor                     string //
	DiscountMinor                     string //
	NetAmountMinor                    string //
	TaxAmountMinor                    string //
	TotalAmountMinor                  string //
	PricingRuleDigest                 string //
	ContentDigest                     string //
	CreatedAt                         string //
	SubjectTypeCode                   string //
	SubjectNo                         string //
	StructuredConfigurationSnapshotNo string //
}

// servicePriceSnapshotColumns holds the columns for the table plugin_idc_core_service_price_snapshot.
var servicePriceSnapshotColumns = ServicePriceSnapshotColumns{
	Id:                                "id",
	TenantId:                          "tenant_id",
	PriceSnapshotNo:                   "price_snapshot_no",
	ServiceNo:                         "service_no",
	CustomerNo:                        "customer_no",
	PhaseCode:                         "phase_code",
	SourceQuoteNo:                     "source_quote_no",
	SourceQuoteItemNo:                 "source_quote_item_no",
	ProductNo:                         "product_no",
	VersionNo:                         "version_no",
	BillingRuleNo:                     "billing_rule_no",
	ConfigurationSnapshotNo:           "configuration_snapshot_no",
	CurrencyCode:                      "currency_code",
	PurchaseCycles:                    "purchase_cycles",
	PeriodUnitCode:                    "period_unit_code",
	PeriodCount:                       "period_count",
	RenewalModeCode:                   "renewal_mode_code",
	CapabilityEvidenceNo:              "capability_evidence_no",
	CapabilityEvidenceDigest:          "capability_evidence_digest",
	CapabilityEvidenceObservedAt:      "capability_evidence_observed_at",
	CapabilityEvidenceExpiresAt:       "capability_evidence_expires_at",
	OriginalExpiresAt:                 "original_expires_at",
	ProjectedExpiresAt:                "projected_expires_at",
	TaxPolicyNo:                       "tax_policy_no",
	PriceIncludesTax:                  "price_includes_tax",
	TaxTreatmentCode:                  "tax_treatment_code",
	TaxRateBasisPoints:                "tax_rate_basis_points",
	TaxCategoryCode:                   "tax_category_code",
	RoundingScopeCode:                 "rounding_scope_code",
	RoundingModeCode:                  "rounding_mode_code",
	RoundingQuantumMinor:              "rounding_quantum_minor",
	ComponentCount:                    "component_count",
	SubtotalMinor:                     "subtotal_minor",
	DiscountMinor:                     "discount_minor",
	NetAmountMinor:                    "net_amount_minor",
	TaxAmountMinor:                    "tax_amount_minor",
	TotalAmountMinor:                  "total_amount_minor",
	PricingRuleDigest:                 "pricing_rule_digest",
	ContentDigest:                     "content_digest",
	CreatedAt:                         "created_at",
	SubjectTypeCode:                   "subject_type_code",
	SubjectNo:                         "subject_no",
	StructuredConfigurationSnapshotNo: "structured_configuration_snapshot_no",
}

// NewServicePriceSnapshotDao creates and returns a new DAO object for table data access.
func NewServicePriceSnapshotDao(handlers ...gdb.ModelHandler) *ServicePriceSnapshotDao {
	return &ServicePriceSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_service_price_snapshot",
		columns:  servicePriceSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServicePriceSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServicePriceSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServicePriceSnapshotDao) Columns() ServicePriceSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServicePriceSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServicePriceSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServicePriceSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
