// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServicePriceComponentSnapshotDao is the data access object for the table plugin_idc_core_service_price_component_snapshot.
type ServicePriceComponentSnapshotDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  ServicePriceComponentSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// ServicePriceComponentSnapshotColumns defines and stores column names for the table plugin_idc_core_service_price_component_snapshot.
type ServicePriceComponentSnapshotColumns struct {
	Id                            string //
	TenantId                      string //
	ComponentSnapshotNo           string //
	PriceSnapshotNo               string //
	ServiceNo                     string //
	LineNumber                    string //
	ComponentTypeCode             string //
	ChargePhaseCode               string //
	OptionNo                      string //
	SourceReference               string //
	Quantity                      string //
	UnitAmountMinor               string //
	AmountMinor                   string //
	SourceDigest                  string //
	ContentDigest                 string //
	CreatedAt                     string //
	SubjectTypeCode               string //
	SubjectNo                     string //
	ConfigurationPriceDimensionNo string //
	ConfigurationSnapshotNo       string //
}

// servicePriceComponentSnapshotColumns holds the columns for the table plugin_idc_core_service_price_component_snapshot.
var servicePriceComponentSnapshotColumns = ServicePriceComponentSnapshotColumns{
	Id:                            "id",
	TenantId:                      "tenant_id",
	ComponentSnapshotNo:           "component_snapshot_no",
	PriceSnapshotNo:               "price_snapshot_no",
	ServiceNo:                     "service_no",
	LineNumber:                    "line_number",
	ComponentTypeCode:             "component_type_code",
	ChargePhaseCode:               "charge_phase_code",
	OptionNo:                      "option_no",
	SourceReference:               "source_reference",
	Quantity:                      "quantity",
	UnitAmountMinor:               "unit_amount_minor",
	AmountMinor:                   "amount_minor",
	SourceDigest:                  "source_digest",
	ContentDigest:                 "content_digest",
	CreatedAt:                     "created_at",
	SubjectTypeCode:               "subject_type_code",
	SubjectNo:                     "subject_no",
	ConfigurationPriceDimensionNo: "configuration_price_dimension_no",
	ConfigurationSnapshotNo:       "configuration_snapshot_no",
}

// NewServicePriceComponentSnapshotDao creates and returns a new DAO object for table data access.
func NewServicePriceComponentSnapshotDao(handlers ...gdb.ModelHandler) *ServicePriceComponentSnapshotDao {
	return &ServicePriceComponentSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_service_price_component_snapshot",
		columns:  servicePriceComponentSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServicePriceComponentSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServicePriceComponentSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServicePriceComponentSnapshotDao) Columns() ServicePriceComponentSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServicePriceComponentSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServicePriceComponentSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServicePriceComponentSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
