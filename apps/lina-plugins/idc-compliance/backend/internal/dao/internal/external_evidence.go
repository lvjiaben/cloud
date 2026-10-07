// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ExternalEvidenceDao is the data access object for the table plugin_idc_compliance_external_evidence.
type ExternalEvidenceDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  ExternalEvidenceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// ExternalEvidenceColumns defines and stores column names for the table plugin_idc_compliance_external_evidence.
type ExternalEvidenceColumns struct {
	Id                string //
	TenantId          string //
	EvidenceNo        string //
	AggregateTypeCode string //
	AggregateNo       string //
	SourceSystemCode  string //
	SourceEventRef    string //
	EvidenceTypeCode  string //
	PayloadSha256     string //
	PrivateObjectRef  string //
	MediaType         string //
	ByteSize          string //
	DispositionCode   string //
	OccurredAt        string //
	ReceivedAt        string //
	CreatedAt         string //
}

// externalEvidenceColumns holds the columns for the table plugin_idc_compliance_external_evidence.
var externalEvidenceColumns = ExternalEvidenceColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	EvidenceNo:        "evidence_no",
	AggregateTypeCode: "aggregate_type_code",
	AggregateNo:       "aggregate_no",
	SourceSystemCode:  "source_system_code",
	SourceEventRef:    "source_event_ref",
	EvidenceTypeCode:  "evidence_type_code",
	PayloadSha256:     "payload_sha256",
	PrivateObjectRef:  "private_object_ref",
	MediaType:         "media_type",
	ByteSize:          "byte_size",
	DispositionCode:   "disposition_code",
	OccurredAt:        "occurred_at",
	ReceivedAt:        "received_at",
	CreatedAt:         "created_at",
}

// NewExternalEvidenceDao creates and returns a new DAO object for table data access.
func NewExternalEvidenceDao(handlers ...gdb.ModelHandler) *ExternalEvidenceDao {
	return &ExternalEvidenceDao{
		group:    "default",
		table:    "plugin_idc_compliance_external_evidence",
		columns:  externalEvidenceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ExternalEvidenceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ExternalEvidenceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ExternalEvidenceDao) Columns() ExternalEvidenceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ExternalEvidenceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ExternalEvidenceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ExternalEvidenceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
