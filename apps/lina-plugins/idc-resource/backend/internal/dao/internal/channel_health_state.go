// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelHealthStateDao is the data access object for the table plugin_idc_resource_channel_health_state.
type ChannelHealthStateDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  ChannelHealthStateColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// ChannelHealthStateColumns defines and stores column names for the table plugin_idc_resource_channel_health_state.
type ChannelHealthStateColumns struct {
	Id                              string //
	TenantId                        string //
	ChannelNo                       string //
	SampleCount                     string //
	FailureCount                    string //
	UnknownCount                    string //
	ConsecutiveFailures             string //
	CircuitOpen                     string //
	WriteProtectedUntil             string // Database-clock protection deadline; read-only task and resource reconciliation remains allowed
	WindowStartedAt                 string //
	LastSummaryCode                 string // Stable non-sensitive machine code; raw upstream errors are forbidden
	LastObservedAt                  string //
	Version                         string //
	CreatedAt                       string //
	UpdatedAt                       string //
	WindowSampleLimit               string // Merchant policy: bounded write sample window from 5 through 100
	MinimumSamplesToProtect         string // Merchant policy: minimum write observations before protection may open
	FailureLimit                    string //
	UnknownResultLimit              string //
	ConsecutiveFailureLimit         string //
	ProtectionDurationSeconds       string // Merchant policy: automatic write protection duration from 60 through 86400 seconds
	PolicyVersion                   string //
	PolicyUpdatedBy                 string //
	PolicyUpdatedAt                 string // PostgreSQL-clock time of the latest audited merchant policy change
	LastAuthenticationHealthCode    string //
	LastAuthenticationSummaryCode   string // Stable non-sensitive latest authentication summary; raw upstream errors are forbidden
	LastAuthenticationLatencyMs     string //
	LastAuthenticationObservedAt    string //
	LastQueryHealthCode             string //
	LastQuerySummaryCode            string // Stable non-sensitive latest query summary; raw upstream errors are forbidden
	LastQueryLatencyMs              string //
	LastQueryObservedAt             string //
	LastWriteHealthCode             string //
	LastWriteSummaryCode            string // Stable non-sensitive latest write summary; raw upstream errors are forbidden
	LastWriteLatencyMs              string //
	LastWriteObservedAt             string //
	LastProviderCapacityHealthCode  string //
	LastProviderCapacitySummaryCode string // Stable non-sensitive latest provider_capacity summary; raw upstream errors are forbidden
	LastProviderCapacityLatencyMs   string //
	LastProviderCapacityObservedAt  string //
}

// channelHealthStateColumns holds the columns for the table plugin_idc_resource_channel_health_state.
var channelHealthStateColumns = ChannelHealthStateColumns{
	Id:                              "id",
	TenantId:                        "tenant_id",
	ChannelNo:                       "channel_no",
	SampleCount:                     "sample_count",
	FailureCount:                    "failure_count",
	UnknownCount:                    "unknown_count",
	ConsecutiveFailures:             "consecutive_failures",
	CircuitOpen:                     "circuit_open",
	WriteProtectedUntil:             "write_protected_until",
	WindowStartedAt:                 "window_started_at",
	LastSummaryCode:                 "last_summary_code",
	LastObservedAt:                  "last_observed_at",
	Version:                         "version",
	CreatedAt:                       "created_at",
	UpdatedAt:                       "updated_at",
	WindowSampleLimit:               "window_sample_limit",
	MinimumSamplesToProtect:         "minimum_samples_to_protect",
	FailureLimit:                    "failure_limit",
	UnknownResultLimit:              "unknown_result_limit",
	ConsecutiveFailureLimit:         "consecutive_failure_limit",
	ProtectionDurationSeconds:       "protection_duration_seconds",
	PolicyVersion:                   "policy_version",
	PolicyUpdatedBy:                 "policy_updated_by",
	PolicyUpdatedAt:                 "policy_updated_at",
	LastAuthenticationHealthCode:    "last_authentication_health_code",
	LastAuthenticationSummaryCode:   "last_authentication_summary_code",
	LastAuthenticationLatencyMs:     "last_authentication_latency_ms",
	LastAuthenticationObservedAt:    "last_authentication_observed_at",
	LastQueryHealthCode:             "last_query_health_code",
	LastQuerySummaryCode:            "last_query_summary_code",
	LastQueryLatencyMs:              "last_query_latency_ms",
	LastQueryObservedAt:             "last_query_observed_at",
	LastWriteHealthCode:             "last_write_health_code",
	LastWriteSummaryCode:            "last_write_summary_code",
	LastWriteLatencyMs:              "last_write_latency_ms",
	LastWriteObservedAt:             "last_write_observed_at",
	LastProviderCapacityHealthCode:  "last_provider_capacity_health_code",
	LastProviderCapacitySummaryCode: "last_provider_capacity_summary_code",
	LastProviderCapacityLatencyMs:   "last_provider_capacity_latency_ms",
	LastProviderCapacityObservedAt:  "last_provider_capacity_observed_at",
}

// NewChannelHealthStateDao creates and returns a new DAO object for table data access.
func NewChannelHealthStateDao(handlers ...gdb.ModelHandler) *ChannelHealthStateDao {
	return &ChannelHealthStateDao{
		group:    "default",
		table:    "plugin_idc_resource_channel_health_state",
		columns:  channelHealthStateColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelHealthStateDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelHealthStateDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelHealthStateDao) Columns() ChannelHealthStateColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelHealthStateDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelHealthStateDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelHealthStateDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
