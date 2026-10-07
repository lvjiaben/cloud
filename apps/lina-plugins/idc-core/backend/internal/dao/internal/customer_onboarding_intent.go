// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerOnboardingIntentDao is the data access object for the table plugin_idc_core_customer_onboarding_intent.
type CustomerOnboardingIntentDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  CustomerOnboardingIntentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// CustomerOnboardingIntentColumns defines and stores column names for the table plugin_idc_core_customer_onboarding_intent.
type CustomerOnboardingIntentColumns struct {
	Id                     string //
	TenantId               string //
	IntentNo               string // Opaque onboarding intent business number
	TokenDigest            string // SHA-256 digest of the single-use opaque token
	BrowserNonceDigest     string // SHA-256 digest of the browser-bound nonce
	NormalizedUsernameHash string // Digest of the normalized requested username
	ContactChannelCode     string //
	ContactHash            string // Digest of the verified contact destination
	CustomerTypeCode       string //
	StatusCode             string //
	ReturnPath             string // Validated same-origin path used after onboarding
	HostRegistrationUserId string // Host identity returned by native registration
	ConsumedByUserId       string //
	ResultCustomerNo       string // Customer business number created by successful redemption
	VerifiedContactAt      string //
	ExpiresAt              string // Intent expiry time
	ConsumedAt             string //
	AttemptCount           string //
	Version                string //
	CreatedAt              string //
	UpdatedAt              string //
	OperationDigest        string //
	RequestDigest          string //
}

// customerOnboardingIntentColumns holds the columns for the table plugin_idc_core_customer_onboarding_intent.
var customerOnboardingIntentColumns = CustomerOnboardingIntentColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	IntentNo:               "intent_no",
	TokenDigest:            "token_digest",
	BrowserNonceDigest:     "browser_nonce_digest",
	NormalizedUsernameHash: "normalized_username_hash",
	ContactChannelCode:     "contact_channel_code",
	ContactHash:            "contact_hash",
	CustomerTypeCode:       "customer_type_code",
	StatusCode:             "status_code",
	ReturnPath:             "return_path",
	HostRegistrationUserId: "host_registration_user_id",
	ConsumedByUserId:       "consumed_by_user_id",
	ResultCustomerNo:       "result_customer_no",
	VerifiedContactAt:      "verified_contact_at",
	ExpiresAt:              "expires_at",
	ConsumedAt:             "consumed_at",
	AttemptCount:           "attempt_count",
	Version:                "version",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
	OperationDigest:        "operation_digest",
	RequestDigest:          "request_digest",
}

// NewCustomerOnboardingIntentDao creates and returns a new DAO object for table data access.
func NewCustomerOnboardingIntentDao(handlers ...gdb.ModelHandler) *CustomerOnboardingIntentDao {
	return &CustomerOnboardingIntentDao{
		group:    "default",
		table:    "plugin_idc_core_customer_onboarding_intent",
		columns:  customerOnboardingIntentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerOnboardingIntentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerOnboardingIntentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerOnboardingIntentDao) Columns() CustomerOnboardingIntentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerOnboardingIntentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerOnboardingIntentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerOnboardingIntentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
