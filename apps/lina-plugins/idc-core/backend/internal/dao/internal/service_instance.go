// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceInstanceDao is the data access object for the table plugin_idc_core_service_instance.
type ServiceInstanceDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  ServiceInstanceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// ServiceInstanceColumns defines and stores column names for the table plugin_idc_core_service_instance.
type ServiceInstanceColumns struct {
	Id                            string //
	TenantId                      string //
	ServiceNo                     string //
	CustomerNo                    string //
	SourceCode                    string //
	SourceFulfillmentNo           string //
	OrderNo                       string //
	OrderItemNo                   string //
	ProductNo                     string //
	VersionNo                     string //
	BillingRuleNo                 string //
	ConfigurationSnapshotNo       string //
	ProductTypeCode               string //
	CurrencyCode                  string //
	ResourceOwnerCode             string //
	BindingNo                     string //
	PoolNo                        string //
	ChannelNo                     string //
	EntitlementStatusCode         string //
	RuntimeStatusCode             string //
	ProviderTaskStatusCode        string //
	RenewalModeCode               string //
	RenewalCapabilityEvidenceNo   string //
	RenewalCapabilityDigest       string //
	RenewalCapabilityObservedAt   string //
	RenewalCapabilityExpiresAt    string //
	PeriodUnitCode                string //
	PeriodCount                   string //
	EntitlementStartedAt          string //
	EntitlementExpiresAt          string //
	CurrentPriceSnapshotNo        string //
	CurrentRenewalPolicyNo        string //
	CurrentRenewalPolicyVersion   string //
	LastBusinessOperationNo       string //
	Version                       string //
	CreatedAt                     string //
	UpdatedAt                     string //
	CurrentLifecyclePolicyNo      string //
	CurrentLifecyclePolicyVersion string //
	CustomerAlias                 string //
	CustomerRemark                string //
	OwnershipStartedAt            string //
}

// serviceInstanceColumns holds the columns for the table plugin_idc_core_service_instance.
var serviceInstanceColumns = ServiceInstanceColumns{
	Id:                            "id",
	TenantId:                      "tenant_id",
	ServiceNo:                     "service_no",
	CustomerNo:                    "customer_no",
	SourceCode:                    "source_code",
	SourceFulfillmentNo:           "source_fulfillment_no",
	OrderNo:                       "order_no",
	OrderItemNo:                   "order_item_no",
	ProductNo:                     "product_no",
	VersionNo:                     "version_no",
	BillingRuleNo:                 "billing_rule_no",
	ConfigurationSnapshotNo:       "configuration_snapshot_no",
	ProductTypeCode:               "product_type_code",
	CurrencyCode:                  "currency_code",
	ResourceOwnerCode:             "resource_owner_code",
	BindingNo:                     "binding_no",
	PoolNo:                        "pool_no",
	ChannelNo:                     "channel_no",
	EntitlementStatusCode:         "entitlement_status_code",
	RuntimeStatusCode:             "runtime_status_code",
	ProviderTaskStatusCode:        "provider_task_status_code",
	RenewalModeCode:               "renewal_mode_code",
	RenewalCapabilityEvidenceNo:   "renewal_capability_evidence_no",
	RenewalCapabilityDigest:       "renewal_capability_digest",
	RenewalCapabilityObservedAt:   "renewal_capability_observed_at",
	RenewalCapabilityExpiresAt:    "renewal_capability_expires_at",
	PeriodUnitCode:                "period_unit_code",
	PeriodCount:                   "period_count",
	EntitlementStartedAt:          "entitlement_started_at",
	EntitlementExpiresAt:          "entitlement_expires_at",
	CurrentPriceSnapshotNo:        "current_price_snapshot_no",
	CurrentRenewalPolicyNo:        "current_renewal_policy_no",
	CurrentRenewalPolicyVersion:   "current_renewal_policy_version",
	LastBusinessOperationNo:       "last_business_operation_no",
	Version:                       "version",
	CreatedAt:                     "created_at",
	UpdatedAt:                     "updated_at",
	CurrentLifecyclePolicyNo:      "current_lifecycle_policy_no",
	CurrentLifecyclePolicyVersion: "current_lifecycle_policy_version",
	CustomerAlias:                 "customer_alias",
	CustomerRemark:                "customer_remark",
	OwnershipStartedAt:            "ownership_started_at",
}

// NewServiceInstanceDao creates and returns a new DAO object for table data access.
func NewServiceInstanceDao(handlers ...gdb.ModelHandler) *ServiceInstanceDao {
	return &ServiceInstanceDao{
		group:    "default",
		table:    "plugin_idc_core_service_instance",
		columns:  serviceInstanceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceInstanceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceInstanceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceInstanceDao) Columns() ServiceInstanceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceInstanceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceInstanceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceInstanceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
