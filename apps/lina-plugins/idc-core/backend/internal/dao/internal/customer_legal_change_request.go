// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerLegalChangeRequestDao is the data access object for the table plugin_idc_core_customer_legal_change_request.
type CustomerLegalChangeRequestDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  CustomerLegalChangeRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// CustomerLegalChangeRequestColumns defines and stores column names for the table plugin_idc_core_customer_legal_change_request.
type CustomerLegalChangeRequestColumns struct {
	Id                            string //
	TenantId                      string //
	RequestNo                     string //
	CustomerNo                    string //
	CustomerTypeCode              string //
	StatusCode                    string //
	ReasonCode                    string //
	ChangeLegalName               string // Declares a requested field category without storing the proposed raw legal value
	ChangeLegalIdentifier         string // Declares a requested identifier category without storing a raw identifier or document
	CurrentVerificationStatusCode string //
	CurrentVerificationCaseNo     string // Opaque reference to the verification case whose result remains unchanged while this request is reviewed
	CurrentProfileVersion         string //
	IdempotencyKeyDigest          string //
	RequestFingerprint            string //
	SubmittedByUserId             string //
	SubmittedAt                   string //
	ReviewedByUserId              string //
	ReviewedAt                    string //
	ResolutionCode                string //
	CreatedAt                     string //
	UpdatedAt                     string //
	ReviewVerificationCaseNo      string // Opaque compliance case performing this legal-data review; proposed values and materials remain compliance-owned
}

// customerLegalChangeRequestColumns holds the columns for the table plugin_idc_core_customer_legal_change_request.
var customerLegalChangeRequestColumns = CustomerLegalChangeRequestColumns{
	Id:                            "id",
	TenantId:                      "tenant_id",
	RequestNo:                     "request_no",
	CustomerNo:                    "customer_no",
	CustomerTypeCode:              "customer_type_code",
	StatusCode:                    "status_code",
	ReasonCode:                    "reason_code",
	ChangeLegalName:               "change_legal_name",
	ChangeLegalIdentifier:         "change_legal_identifier",
	CurrentVerificationStatusCode: "current_verification_status_code",
	CurrentVerificationCaseNo:     "current_verification_case_no",
	CurrentProfileVersion:         "current_profile_version",
	IdempotencyKeyDigest:          "idempotency_key_digest",
	RequestFingerprint:            "request_fingerprint",
	SubmittedByUserId:             "submitted_by_user_id",
	SubmittedAt:                   "submitted_at",
	ReviewedByUserId:              "reviewed_by_user_id",
	ReviewedAt:                    "reviewed_at",
	ResolutionCode:                "resolution_code",
	CreatedAt:                     "created_at",
	UpdatedAt:                     "updated_at",
	ReviewVerificationCaseNo:      "review_verification_case_no",
}

// NewCustomerLegalChangeRequestDao creates and returns a new DAO object for table data access.
func NewCustomerLegalChangeRequestDao(handlers ...gdb.ModelHandler) *CustomerLegalChangeRequestDao {
	return &CustomerLegalChangeRequestDao{
		group:    "default",
		table:    "plugin_idc_core_customer_legal_change_request",
		columns:  customerLegalChangeRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerLegalChangeRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerLegalChangeRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerLegalChangeRequestDao) Columns() CustomerLegalChangeRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerLegalChangeRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerLegalChangeRequestDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerLegalChangeRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
