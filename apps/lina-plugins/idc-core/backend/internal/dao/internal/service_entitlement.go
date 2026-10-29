// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceEntitlementDao is the data access object for the table plugin_idc_core_service_entitlement.
type ServiceEntitlementDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  ServiceEntitlementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// ServiceEntitlementColumns defines and stores column names for the table plugin_idc_core_service_entitlement.
type ServiceEntitlementColumns struct {
	TenantId                    string //
	Id                          string //
	EntitlementNo               string //
	DeliveryNo                  string //
	SourceIntentNo              string //
	CustomerNo                  string //
	OrderNo                     string //
	OrderItemNo                 string //
	ProductNo                   string //
	VersionNo                   string //
	UnitSequence                string //
	ConfigurationSnapshotNo     string //
	InterfaceInstanceNo         string //
	PluginId                    string //
	PluginReleaseVersion        string //
	ContractVersion             string //
	DeclarationDigest           string //
	StatusCode                  string //
	StartedAt                   string //
	ExpiresAt                   string //
	Version                     string //
	CreatedAt                   string //
	UpdatedAt                   string //
	RenewalModeCode             string //
	RenewalMethod               string //
	EntitlementOwnerId          string //
	CurrencyCode                string //
	BillingRuleNo               string //
	ChargeTypeCode              string //
	PeriodUnitCode              string //
	PeriodCount                 string //
	CurrentRenewalPolicyNo      string //
	CurrentRenewalPolicyVersion string //
	CurrentPriceSnapshotNo      string //
	LastRenewalOperationNo      string //
	SupportAssociationReference string //
	UpstreamOrderReference      string //
	UpstreamDeliveryNo          string //
	UpstreamServiceNo           string //
	CustomerRemark              string // Current customer non-sensitive service note
	CustomerRemarkRevision      string // Independent optimistic revision for the customer service note
}

// serviceEntitlementColumns holds the columns for the table plugin_idc_core_service_entitlement.
var serviceEntitlementColumns = ServiceEntitlementColumns{
	TenantId:                    "tenant_id",
	Id:                          "id",
	EntitlementNo:               "entitlement_no",
	DeliveryNo:                  "delivery_no",
	SourceIntentNo:              "source_intent_no",
	CustomerNo:                  "customer_no",
	OrderNo:                     "order_no",
	OrderItemNo:                 "order_item_no",
	ProductNo:                   "product_no",
	VersionNo:                   "version_no",
	UnitSequence:                "unit_sequence",
	ConfigurationSnapshotNo:     "configuration_snapshot_no",
	InterfaceInstanceNo:         "interface_instance_no",
	PluginId:                    "plugin_id",
	PluginReleaseVersion:        "plugin_release_version",
	ContractVersion:             "contract_version",
	DeclarationDigest:           "declaration_digest",
	StatusCode:                  "status_code",
	StartedAt:                   "started_at",
	ExpiresAt:                   "expires_at",
	Version:                     "version",
	CreatedAt:                   "created_at",
	UpdatedAt:                   "updated_at",
	RenewalModeCode:             "renewal_mode_code",
	RenewalMethod:               "renewal_method",
	EntitlementOwnerId:          "entitlement_owner_id",
	CurrencyCode:                "currency_code",
	BillingRuleNo:               "billing_rule_no",
	ChargeTypeCode:              "charge_type_code",
	PeriodUnitCode:              "period_unit_code",
	PeriodCount:                 "period_count",
	CurrentRenewalPolicyNo:      "current_renewal_policy_no",
	CurrentRenewalPolicyVersion: "current_renewal_policy_version",
	CurrentPriceSnapshotNo:      "current_price_snapshot_no",
	LastRenewalOperationNo:      "last_renewal_operation_no",
	SupportAssociationReference: "support_association_reference",
	UpstreamOrderReference:      "upstream_order_reference",
	UpstreamDeliveryNo:          "upstream_delivery_no",
	UpstreamServiceNo:           "upstream_service_no",
	CustomerRemark:              "customer_remark",
	CustomerRemarkRevision:      "customer_remark_revision",
}

// NewServiceEntitlementDao creates and returns a new DAO object for table data access.
func NewServiceEntitlementDao(handlers ...gdb.ModelHandler) *ServiceEntitlementDao {
	return &ServiceEntitlementDao{
		group:    "default",
		table:    "plugin_idc_core_service_entitlement",
		columns:  serviceEntitlementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceEntitlementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceEntitlementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceEntitlementDao) Columns() ServiceEntitlementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceEntitlementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceEntitlementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceEntitlementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
