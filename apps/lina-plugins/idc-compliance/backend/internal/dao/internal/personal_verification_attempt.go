// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PersonalVerificationAttemptDao is the data access object for the table plugin_idc_compliance_personal_verification_attempt.
type PersonalVerificationAttemptDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  PersonalVerificationAttemptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// PersonalVerificationAttemptColumns defines and stores column names for the table plugin_idc_compliance_personal_verification_attempt.
type PersonalVerificationAttemptColumns struct {
	Id                        string //
	TenantId                  string //
	AttemptNo                 string //
	CaseNo                    string //
	SubjectTypeCode           string //
	ConfigurationNo           string //
	ProviderCode              string //
	ConfigurationVersion      string //
	OuterOrderNo              string //
	CertifyId                 string // Aliyun billing and reconciliation identifier; certify URL, face images, face videos, identity plaintext, and raw provider payloads are forbidden
	StatusCode                string //
	LastProviderResultCode    string //
	NotPassedObservationCount string //
	InitializationStartedAt   string //
	IssuedAt                  string //
	CertifyExpiresAt          string //
	LastQueriedAt             string //
	PassedAt                  string //
	ExpiredAt                 string //
	Version                   string //
	CreatedAt                 string //
	UpdatedAt                 string //
}

// personalVerificationAttemptColumns holds the columns for the table plugin_idc_compliance_personal_verification_attempt.
var personalVerificationAttemptColumns = PersonalVerificationAttemptColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	AttemptNo:                 "attempt_no",
	CaseNo:                    "case_no",
	SubjectTypeCode:           "subject_type_code",
	ConfigurationNo:           "configuration_no",
	ProviderCode:              "provider_code",
	ConfigurationVersion:      "configuration_version",
	OuterOrderNo:              "outer_order_no",
	CertifyId:                 "certify_id",
	StatusCode:                "status_code",
	LastProviderResultCode:    "last_provider_result_code",
	NotPassedObservationCount: "not_passed_observation_count",
	InitializationStartedAt:   "initialization_started_at",
	IssuedAt:                  "issued_at",
	CertifyExpiresAt:          "certify_expires_at",
	LastQueriedAt:             "last_queried_at",
	PassedAt:                  "passed_at",
	ExpiredAt:                 "expired_at",
	Version:                   "version",
	CreatedAt:                 "created_at",
	UpdatedAt:                 "updated_at",
}

// NewPersonalVerificationAttemptDao creates and returns a new DAO object for table data access.
func NewPersonalVerificationAttemptDao(handlers ...gdb.ModelHandler) *PersonalVerificationAttemptDao {
	return &PersonalVerificationAttemptDao{
		group:    "default",
		table:    "plugin_idc_compliance_personal_verification_attempt",
		columns:  personalVerificationAttemptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PersonalVerificationAttemptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PersonalVerificationAttemptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PersonalVerificationAttemptDao) Columns() PersonalVerificationAttemptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PersonalVerificationAttemptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PersonalVerificationAttemptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PersonalVerificationAttemptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
