// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContactVerificationDao is the data access object for the table plugin_idc_notify_contact_verification.
type ContactVerificationDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  ContactVerificationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// ContactVerificationColumns defines and stores column names for the table plugin_idc_notify_contact_verification.
type ContactVerificationColumns struct {
	TenantId                string //
	VerificationNo          string //
	PurposeCode             string //
	OperationCode           string //
	DestinationKind         string //
	DestinationDigest       string //
	BrowserDigest           string //
	TicketDigest            string //
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
	RetryAfterAt            string //
	ExpiresAt               string //
	ProviderBoundaryCrossed string //
	VerifiedAt              string //
	CreatedAt               string //
	UpdatedAt               string //
}

// contactVerificationColumns holds the columns for the table plugin_idc_notify_contact_verification.
var contactVerificationColumns = ContactVerificationColumns{
	TenantId:                "tenant_id",
	VerificationNo:          "verification_no",
	PurposeCode:             "purpose_code",
	OperationCode:           "operation_code",
	DestinationKind:         "destination_kind",
	DestinationDigest:       "destination_digest",
	BrowserDigest:           "browser_digest",
	TicketDigest:            "ticket_digest",
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
	RetryAfterAt:            "retry_after_at",
	ExpiresAt:               "expires_at",
	ProviderBoundaryCrossed: "provider_boundary_crossed",
	VerifiedAt:              "verified_at",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewContactVerificationDao creates and returns a new DAO object for table data access.
func NewContactVerificationDao(handlers ...gdb.ModelHandler) *ContactVerificationDao {
	return &ContactVerificationDao{
		group:    "default",
		table:    "plugin_idc_notify_contact_verification",
		columns:  contactVerificationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContactVerificationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContactVerificationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContactVerificationDao) Columns() ContactVerificationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContactVerificationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContactVerificationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContactVerificationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
