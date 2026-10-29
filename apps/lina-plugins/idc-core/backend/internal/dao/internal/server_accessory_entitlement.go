// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryEntitlementDao is the data access object for the table plugin_idc_core_server_accessory_entitlement.
type ServerAccessoryEntitlementDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  ServerAccessoryEntitlementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// ServerAccessoryEntitlementColumns defines and stores column names for the table plugin_idc_core_server_accessory_entitlement.
type ServerAccessoryEntitlementColumns struct {
	Id                      string //
	TenantId                string //
	EntitlementNo           string //
	CustomerNo              string //
	ServiceNo               string //
	CreateOperationNo       string //
	ObjectNo                string // Resource-owner opaque object number; Provider identifiers never cross this boundary
	AccessoryKind           string //
	RequestedName           string //
	BillingModeCode         string //
	StatusCode              string //
	AutoRenewEnabled        string //
	CurrencyCode            string //
	PeriodUnitCode          string //
	PeriodCount             string //
	CurrentPeriodStartAt    string //
	CurrentPeriodEndAt      string //
	PendingInvoiceNo        string //
	PendingPolicyNo         string //
	PendingPolicyVersion    string //
	PendingPeriodUnitCode   string //
	PendingPeriodCount      string //
	PendingTotalAmountMinor string //
	LastRenewalAttemptAt    string //
	BaselineObjectNos       string //
	BaselineSnapshotNo      string //
	BaselineObservedAt      string //
	BindingAttemptCount     string //
	LastBindingAttemptAt    string //
	DeletedAt               string //
	Version                 string //
	CreatedAt               string //
	UpdatedAt               string //
	SubjectTypeCode         string //
	SubjectNo               string //
}

// serverAccessoryEntitlementColumns holds the columns for the table plugin_idc_core_server_accessory_entitlement.
var serverAccessoryEntitlementColumns = ServerAccessoryEntitlementColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	EntitlementNo:           "entitlement_no",
	CustomerNo:              "customer_no",
	ServiceNo:               "service_no",
	CreateOperationNo:       "create_operation_no",
	ObjectNo:                "object_no",
	AccessoryKind:           "accessory_kind",
	RequestedName:           "requested_name",
	BillingModeCode:         "billing_mode_code",
	StatusCode:              "status_code",
	AutoRenewEnabled:        "auto_renew_enabled",
	CurrencyCode:            "currency_code",
	PeriodUnitCode:          "period_unit_code",
	PeriodCount:             "period_count",
	CurrentPeriodStartAt:    "current_period_start_at",
	CurrentPeriodEndAt:      "current_period_end_at",
	PendingInvoiceNo:        "pending_invoice_no",
	PendingPolicyNo:         "pending_policy_no",
	PendingPolicyVersion:    "pending_policy_version",
	PendingPeriodUnitCode:   "pending_period_unit_code",
	PendingPeriodCount:      "pending_period_count",
	PendingTotalAmountMinor: "pending_total_amount_minor",
	LastRenewalAttemptAt:    "last_renewal_attempt_at",
	BaselineObjectNos:       "baseline_object_nos",
	BaselineSnapshotNo:      "baseline_snapshot_no",
	BaselineObservedAt:      "baseline_observed_at",
	BindingAttemptCount:     "binding_attempt_count",
	LastBindingAttemptAt:    "last_binding_attempt_at",
	DeletedAt:               "deleted_at",
	Version:                 "version",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
	SubjectTypeCode:         "subject_type_code",
	SubjectNo:               "subject_no",
}

// NewServerAccessoryEntitlementDao creates and returns a new DAO object for table data access.
func NewServerAccessoryEntitlementDao(handlers ...gdb.ModelHandler) *ServerAccessoryEntitlementDao {
	return &ServerAccessoryEntitlementDao{
		group:    "default",
		table:    "plugin_idc_core_server_accessory_entitlement",
		columns:  serverAccessoryEntitlementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerAccessoryEntitlementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerAccessoryEntitlementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerAccessoryEntitlementDao) Columns() ServerAccessoryEntitlementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerAccessoryEntitlementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerAccessoryEntitlementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerAccessoryEntitlementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
