// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractDao is the data access object for the table plugin_idc_compliance_contract.
type ContractDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ContractColumns    // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ContractColumns defines and stores column names for the table plugin_idc_compliance_contract.
type ContractColumns struct {
	Id                        string //
	TenantId                  string //
	ContractNo                string //
	CustomerNo                string //
	PrincipalNo               string //
	OrderNo                   string //
	ServiceNo                 string //
	ContractTypeCode          string //
	RequiredForFulfillment    string //
	TemplateNo                string //
	TemplateVersion           string //
	VariableSnapshotRef       string //
	VariableSnapshotSha256    string //
	SignerSnapshotSha256      string //
	CurrencyCode              string //
	AmountMinor               string //
	StatusCode                string //
	LaunchedAt                string //
	CompletedAt               string //
	TerminatedAt              string //
	Version                   string //
	CreatedAt                 string //
	UpdatedAt                 string //
	SubjectTypeCode           string //
	SigningOrderCode          string //
	SnapshotKeyRef            string // Opaque secretcap handle for the encrypted contract preparation bundle
	OrderSnapshotSha256       string // Digest of the canonical immutable core order, service, identity, and amount snapshot
	CreationIdempotencyDigest string //
	CreationFingerprint       string //
	SigningExpiresAt          string // Fixed signing deadline exactly seven days after authoritative contract creation
}

// contractColumns holds the columns for the table plugin_idc_compliance_contract.
var contractColumns = ContractColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	ContractNo:                "contract_no",
	CustomerNo:                "customer_no",
	PrincipalNo:               "principal_no",
	OrderNo:                   "order_no",
	ServiceNo:                 "service_no",
	ContractTypeCode:          "contract_type_code",
	RequiredForFulfillment:    "required_for_fulfillment",
	TemplateNo:                "template_no",
	TemplateVersion:           "template_version",
	VariableSnapshotRef:       "variable_snapshot_ref",
	VariableSnapshotSha256:    "variable_snapshot_sha256",
	SignerSnapshotSha256:      "signer_snapshot_sha256",
	CurrencyCode:              "currency_code",
	AmountMinor:               "amount_minor",
	StatusCode:                "status_code",
	LaunchedAt:                "launched_at",
	CompletedAt:               "completed_at",
	TerminatedAt:              "terminated_at",
	Version:                   "version",
	CreatedAt:                 "created_at",
	UpdatedAt:                 "updated_at",
	SubjectTypeCode:           "subject_type_code",
	SigningOrderCode:          "signing_order_code",
	SnapshotKeyRef:            "snapshot_key_ref",
	OrderSnapshotSha256:       "order_snapshot_sha256",
	CreationIdempotencyDigest: "creation_idempotency_digest",
	CreationFingerprint:       "creation_fingerprint",
	SigningExpiresAt:          "signing_expires_at",
}

// NewContractDao creates and returns a new DAO object for table data access.
func NewContractDao(handlers ...gdb.ModelHandler) *ContractDao {
	return &ContractDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract",
		columns:  contractColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractDao) Columns() ContractColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
