// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyIncidentEventDao is the data access object for the table plugin_idc_compliance_privacy_incident_event.
type PrivacyIncidentEventDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  PrivacyIncidentEventColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// PrivacyIncidentEventColumns defines and stores column names for the table plugin_idc_compliance_privacy_incident_event.
type PrivacyIncidentEventColumns struct {
	Id          string //
	TenantId    string //
	EventNo     string //
	IncidentNo  string //
	StageCode   string //
	SafeNote    string //
	ActorUserId string //
	OccurredAt  string //
	CreatedAt   string //
}

// privacyIncidentEventColumns holds the columns for the table plugin_idc_compliance_privacy_incident_event.
var privacyIncidentEventColumns = PrivacyIncidentEventColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	EventNo:     "event_no",
	IncidentNo:  "incident_no",
	StageCode:   "stage_code",
	SafeNote:    "safe_note",
	ActorUserId: "actor_user_id",
	OccurredAt:  "occurred_at",
	CreatedAt:   "created_at",
}

// NewPrivacyIncidentEventDao creates and returns a new DAO object for table data access.
func NewPrivacyIncidentEventDao(handlers ...gdb.ModelHandler) *PrivacyIncidentEventDao {
	return &PrivacyIncidentEventDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_incident_event",
		columns:  privacyIncidentEventColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacyIncidentEventDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacyIncidentEventDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacyIncidentEventDao) Columns() PrivacyIncidentEventColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacyIncidentEventDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacyIncidentEventDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrivacyIncidentEventDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
