// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentScanAttemptDao is the data access object for the table plugin_idc_core_content_scan_attempt.
type ContentScanAttemptDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  ContentScanAttemptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// ContentScanAttemptColumns defines and stores column names for the table plugin_idc_core_content_scan_attempt.
type ContentScanAttemptColumns struct {
	Id                  string //
	TenantId            string //
	AttemptNo           string //
	FileNo              string //
	CandidateGeneration string //
	StatusCode          string //
	QueuedAt            string //
	StartedAt           string //
	LeaseExpiresAt      string //
	FinishedAt          string //
	RiskCode            string //
	CreatedBy           string //
	Version             string //
}

// contentScanAttemptColumns holds the columns for the table plugin_idc_core_content_scan_attempt.
var contentScanAttemptColumns = ContentScanAttemptColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	AttemptNo:           "attempt_no",
	FileNo:              "file_no",
	CandidateGeneration: "candidate_generation",
	StatusCode:          "status_code",
	QueuedAt:            "queued_at",
	StartedAt:           "started_at",
	LeaseExpiresAt:      "lease_expires_at",
	FinishedAt:          "finished_at",
	RiskCode:            "risk_code",
	CreatedBy:           "created_by",
	Version:             "version",
}

// NewContentScanAttemptDao creates and returns a new DAO object for table data access.
func NewContentScanAttemptDao(handlers ...gdb.ModelHandler) *ContentScanAttemptDao {
	return &ContentScanAttemptDao{
		group:    "default",
		table:    "plugin_idc_core_content_scan_attempt",
		columns:  contentScanAttemptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentScanAttemptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentScanAttemptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentScanAttemptDao) Columns() ContentScanAttemptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentScanAttemptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentScanAttemptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentScanAttemptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
