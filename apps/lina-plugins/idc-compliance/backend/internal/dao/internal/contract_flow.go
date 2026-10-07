// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractFlowDao is the data access object for the table plugin_idc_compliance_contract_flow.
type ContractFlowDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  ContractFlowColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// ContractFlowColumns defines and stores column names for the table plugin_idc_compliance_contract_flow.
type ContractFlowColumns struct {
	Id                    string //
	TenantId              string //
	FlowNo                string //
	ContractNo            string //
	SequenceNo            string //
	PurposeCode           string //
	ProviderCode          string //
	ExternalFlowRef       string //
	IdempotencySha256     string //
	RequestSnapshotRef    string //
	RequestSnapshotSha256 string //
	StatusCode            string //
	AvailableAt           string //
	NextQueryAt           string //
	ExpiresAt             string //
	SubmittedAt           string //
	CompletedAt           string //
	FailureCode           string //
	Version               string //
	CreatedAt             string //
	UpdatedAt             string //
	GatewayRouteNo        string // Opaque installation-local supplier-write route; never a Tencent flow identifier
	ProviderStatus        string //
	GatewayRouteVersion   string //
}

// contractFlowColumns holds the columns for the table plugin_idc_compliance_contract_flow.
var contractFlowColumns = ContractFlowColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	FlowNo:                "flow_no",
	ContractNo:            "contract_no",
	SequenceNo:            "sequence_no",
	PurposeCode:           "purpose_code",
	ProviderCode:          "provider_code",
	ExternalFlowRef:       "external_flow_ref",
	IdempotencySha256:     "idempotency_sha256",
	RequestSnapshotRef:    "request_snapshot_ref",
	RequestSnapshotSha256: "request_snapshot_sha256",
	StatusCode:            "status_code",
	AvailableAt:           "available_at",
	NextQueryAt:           "next_query_at",
	ExpiresAt:             "expires_at",
	SubmittedAt:           "submitted_at",
	CompletedAt:           "completed_at",
	FailureCode:           "failure_code",
	Version:               "version",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	GatewayRouteNo:        "gateway_route_no",
	ProviderStatus:        "provider_status",
	GatewayRouteVersion:   "gateway_route_version",
}

// NewContractFlowDao creates and returns a new DAO object for table data access.
func NewContractFlowDao(handlers ...gdb.ModelHandler) *ContractFlowDao {
	return &ContractFlowDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_flow",
		columns:  contractFlowColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractFlowDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractFlowDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractFlowDao) Columns() ContractFlowColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractFlowDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractFlowDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractFlowDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
