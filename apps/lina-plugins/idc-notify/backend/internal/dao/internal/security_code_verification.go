// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SecurityCodeVerificationDao is the data access object for the table plugin_idc_notify_security_code_verification.
type SecurityCodeVerificationDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  SecurityCodeVerificationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// SecurityCodeVerificationColumns defines and stores column names for the table plugin_idc_notify_security_code_verification.
type SecurityCodeVerificationColumns struct {
	TenantId                string //
	VerificationNo          string //
	CallerPluginId          string //
	PurposeCode             string //
	PurposeVersion          string //
	SubjectDigest           string //
	DestinationDigest       string //
	BrowserDigest           string //
	LocaleCode              string //
	TemplateNo              string //
	TemplateVersion         string //
	RequestShapeDigest      string //
	IdempotencyDigest       string //
	CodeDigest              string //
	MaskedDestination       string //
	StatusCode              string //
	DeliveryState           string //
	FailedAttempts          string //
	VerificationVersion     string //
	RetryAfterAt            string //
	ExpiresAt               string //
	ProviderBoundaryCrossed string //
	VerifiedAt              string //
	CreatedAt               string //
	UpdatedAt               string //
}

// securityCodeVerificationColumns holds the columns for the table plugin_idc_notify_security_code_verification.
var securityCodeVerificationColumns = SecurityCodeVerificationColumns{
	TenantId:                "tenant_id",
	VerificationNo:          "verification_no",
	CallerPluginId:          "caller_plugin_id",
	PurposeCode:             "purpose_code",
	PurposeVersion:          "purpose_version",
	SubjectDigest:           "subject_digest",
	DestinationDigest:       "destination_digest",
	BrowserDigest:           "browser_digest",
	LocaleCode:              "locale_code",
	TemplateNo:              "template_no",
	TemplateVersion:         "template_version",
	RequestShapeDigest:      "request_shape_digest",
	IdempotencyDigest:       "idempotency_digest",
	CodeDigest:              "code_digest",
	MaskedDestination:       "masked_destination",
	StatusCode:              "status_code",
	DeliveryState:           "delivery_state",
	FailedAttempts:          "failed_attempts",
	VerificationVersion:     "verification_version",
	RetryAfterAt:            "retry_after_at",
	ExpiresAt:               "expires_at",
	ProviderBoundaryCrossed: "provider_boundary_crossed",
	VerifiedAt:              "verified_at",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewSecurityCodeVerificationDao creates and returns a new DAO object for table data access.
func NewSecurityCodeVerificationDao(handlers ...gdb.ModelHandler) *SecurityCodeVerificationDao {
	return &SecurityCodeVerificationDao{
		group:    "default",
		table:    "plugin_idc_notify_security_code_verification",
		columns:  securityCodeVerificationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SecurityCodeVerificationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SecurityCodeVerificationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SecurityCodeVerificationDao) Columns() SecurityCodeVerificationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SecurityCodeVerificationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SecurityCodeVerificationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SecurityCodeVerificationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
