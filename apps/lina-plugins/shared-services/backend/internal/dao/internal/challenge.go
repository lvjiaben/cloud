// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChallengeDao is the data access object for the table plugin_shared_services_challenge.
type ChallengeDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ChallengeColumns   // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ChallengeColumns defines and stores column names for the table plugin_shared_services_challenge.
type ChallengeColumns struct {
	Id               string //
	TenantId         string //
	ChallengeNo      string //
	CallerPluginId   string //
	Purpose          string //
	BindingDigest    string //
	ProviderCode     string //
	PresentationKind string //
	AnswerSalt       string //
	AnswerDigest     string //
	ProofDigest      string //
	Status           string //
	AttemptCount     string //
	MaxAttempts      string //
	ExpiresAt        string //
	ProofExpiresAt   string //
	VerifiedAt       string //
	ConsumedAt       string //
	CreatedAt        string //
	UpdatedAt        string //
}

// challengeColumns holds the columns for the table plugin_shared_services_challenge.
var challengeColumns = ChallengeColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	ChallengeNo:      "challenge_no",
	CallerPluginId:   "caller_plugin_id",
	Purpose:          "purpose",
	BindingDigest:    "binding_digest",
	ProviderCode:     "provider_code",
	PresentationKind: "presentation_kind",
	AnswerSalt:       "answer_salt",
	AnswerDigest:     "answer_digest",
	ProofDigest:      "proof_digest",
	Status:           "status",
	AttemptCount:     "attempt_count",
	MaxAttempts:      "max_attempts",
	ExpiresAt:        "expires_at",
	ProofExpiresAt:   "proof_expires_at",
	VerifiedAt:       "verified_at",
	ConsumedAt:       "consumed_at",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewChallengeDao creates and returns a new DAO object for table data access.
func NewChallengeDao(handlers ...gdb.ModelHandler) *ChallengeDao {
	return &ChallengeDao{
		group:    "default",
		table:    "plugin_shared_services_challenge",
		columns:  challengeColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChallengeDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChallengeDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChallengeDao) Columns() ChallengeColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChallengeDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChallengeDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChallengeDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
