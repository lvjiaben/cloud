// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractSignerProfileDao is the data access object for the table plugin_idc_compliance_contract_signer_profile.
type ContractSignerProfileDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  ContractSignerProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// ContractSignerProfileColumns defines and stores column names for the table plugin_idc_compliance_contract_signer_profile.
type ContractSignerProfileColumns struct {
	Id                          string //
	TenantId                    string //
	SignerNo                    string //
	CustomerNo                  string //
	ProfileSnapshotRef          string //
	ProfileSnapshotSha256       string //
	MaskedName                  string //
	MaskedIdentityNo            string //
	MaskedPhone                 string //
	ConsentPolicyVersion        string //
	ConsentOccurredAt           string //
	IdentityStatusCode          string //
	IdentityAttemptNo           string //
	IdentityVerificationVersion string //
	IdentityVerifiedAt          string //
	PhoneStatusCode             string //
	PhoneVerificationNo         string //
	PhoneVerificationVersion    string //
	PhoneVerifiedAt             string //
	StatusCode                  string //
	AuthorizationVersion        string //
	CreationDigest              string //
	DisabledAt                  string //
	Version                     string //
	CreatedAt                   string //
	UpdatedAt                   string //
}

// contractSignerProfileColumns holds the columns for the table plugin_idc_compliance_contract_signer_profile.
var contractSignerProfileColumns = ContractSignerProfileColumns{
	Id:                          "id",
	TenantId:                    "tenant_id",
	SignerNo:                    "signer_no",
	CustomerNo:                  "customer_no",
	ProfileSnapshotRef:          "profile_snapshot_ref",
	ProfileSnapshotSha256:       "profile_snapshot_sha256",
	MaskedName:                  "masked_name",
	MaskedIdentityNo:            "masked_identity_no",
	MaskedPhone:                 "masked_phone",
	ConsentPolicyVersion:        "consent_policy_version",
	ConsentOccurredAt:           "consent_occurred_at",
	IdentityStatusCode:          "identity_status_code",
	IdentityAttemptNo:           "identity_attempt_no",
	IdentityVerificationVersion: "identity_verification_version",
	IdentityVerifiedAt:          "identity_verified_at",
	PhoneStatusCode:             "phone_status_code",
	PhoneVerificationNo:         "phone_verification_no",
	PhoneVerificationVersion:    "phone_verification_version",
	PhoneVerifiedAt:             "phone_verified_at",
	StatusCode:                  "status_code",
	AuthorizationVersion:        "authorization_version",
	CreationDigest:              "creation_digest",
	DisabledAt:                  "disabled_at",
	Version:                     "version",
	CreatedAt:                   "created_at",
	UpdatedAt:                   "updated_at",
}

// NewContractSignerProfileDao creates and returns a new DAO object for table data access.
func NewContractSignerProfileDao(handlers ...gdb.ModelHandler) *ContractSignerProfileDao {
	return &ContractSignerProfileDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_signer_profile",
		columns:  contractSignerProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractSignerProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractSignerProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractSignerProfileDao) Columns() ContractSignerProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractSignerProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractSignerProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractSignerProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
