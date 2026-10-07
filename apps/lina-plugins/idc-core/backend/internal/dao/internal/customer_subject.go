// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerSubjectDao is the data access object for the table plugin_idc_core_customer_subject.
type CustomerSubjectDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  CustomerSubjectColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// CustomerSubjectColumns defines and stores column names for the table plugin_idc_core_customer_subject.
type CustomerSubjectColumns struct {
	Id                           string //
	TenantId                     string //
	CustomerNo                   string // Opaque public customer business number
	CustomerTypeCode             string // Customer type code: individual or enterprise
	StatusCode                   string // Customer lifecycle status code
	DisplayName                  string // Customer display name without verified legal identity evidence
	CountryCode                  string //
	PreferredLocale              string // Preferred IETF locale tag
	Version                      string //
	AnonymizedAt                 string // Time when approved account closure anonymization completed
	CreatedBy                    string //
	UpdatedBy                    string //
	CreatedAt                    string //
	UpdatedAt                    string //
	SearchName                   string // Service-normalized display-name projection for indexed prefix search
	StatusReasonCode             string // Stable reason code for the current account restriction or lifecycle state
	StatusReason                 string // Bounded operator explanation presented through localized business copy
	StatusChangedBy              string //
	StatusChangedAt              string //
	EnterpriseContractPolicyCode string // Enterprise default: none or required; it affects only orders created after the setting changes
	EnterpriseContractTemplateNo string //
	CustomerCode                 string // Short monotonic customer number shown to staff and customers; starts at 10000 and is never reused
}

// customerSubjectColumns holds the columns for the table plugin_idc_core_customer_subject.
var customerSubjectColumns = CustomerSubjectColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	CustomerNo:                   "customer_no",
	CustomerTypeCode:             "customer_type_code",
	StatusCode:                   "status_code",
	DisplayName:                  "display_name",
	CountryCode:                  "country_code",
	PreferredLocale:              "preferred_locale",
	Version:                      "version",
	AnonymizedAt:                 "anonymized_at",
	CreatedBy:                    "created_by",
	UpdatedBy:                    "updated_by",
	CreatedAt:                    "created_at",
	UpdatedAt:                    "updated_at",
	SearchName:                   "search_name",
	StatusReasonCode:             "status_reason_code",
	StatusReason:                 "status_reason",
	StatusChangedBy:              "status_changed_by",
	StatusChangedAt:              "status_changed_at",
	EnterpriseContractPolicyCode: "enterprise_contract_policy_code",
	EnterpriseContractTemplateNo: "enterprise_contract_template_no",
	CustomerCode:                 "customer_code",
}

// NewCustomerSubjectDao creates and returns a new DAO object for table data access.
func NewCustomerSubjectDao(handlers ...gdb.ModelHandler) *CustomerSubjectDao {
	return &CustomerSubjectDao{
		group:    "default",
		table:    "plugin_idc_core_customer_subject",
		columns:  customerSubjectColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerSubjectDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerSubjectDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerSubjectDao) Columns() CustomerSubjectColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerSubjectDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerSubjectDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerSubjectDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
