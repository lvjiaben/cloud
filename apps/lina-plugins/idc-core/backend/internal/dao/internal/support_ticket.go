// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketDao is the data access object for the table plugin_idc_core_support_ticket.
type SupportTicketDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SupportTicketColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SupportTicketColumns defines and stores column names for the table plugin_idc_core_support_ticket.
type SupportTicketColumns struct {
	Id                         string //
	TenantId                   string //
	TicketNo                   string //
	TicketType                 string //
	VisibilityCode             string //
	CustomerNo                 string //
	RelatedEntitlementNo       string // Immutable optional customer service reference revalidated at creation; intentionally not a foreign key so ticket history survives service archival
	RelatedEntitlementVersion  string //
	RelatedEntitlementNameZhCn string //
	RelatedEntitlementNameEnUs string //
	RelatedEntitlementStatus   string //
	AnonymousDestinationDigest string //
	AnonymousDestinationKind   string //
	CategoryNo                 string //
	DepartmentNo               string //
	AssigneeUserId             string //
	Subject                    string //
	BaseStage                  string //
	CustomStatusNo             string //
	PriorityCode               string //
	CurrentRound               string //
	FollowUpRequired           string //
	SlaRiskCode                string //
	ResolvedAt                 string //
	ClosedAt                   string //
	ClosedReasonCode           string //
	ClosedByType               string //
	CustomerLastRepliedAt      string //
	LastActivityAt             string //
	Version                    string //
	CreatedBy                  string //
	UpdatedBy                  string //
	CreatedAt                  string //
	UpdatedAt                  string //
}

// supportTicketColumns holds the columns for the table plugin_idc_core_support_ticket.
var supportTicketColumns = SupportTicketColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	TicketNo:                   "ticket_no",
	TicketType:                 "ticket_type",
	VisibilityCode:             "visibility_code",
	CustomerNo:                 "customer_no",
	RelatedEntitlementNo:       "related_entitlement_no",
	RelatedEntitlementVersion:  "related_entitlement_version",
	RelatedEntitlementNameZhCn: "related_entitlement_name_zh_cn",
	RelatedEntitlementNameEnUs: "related_entitlement_name_en_us",
	RelatedEntitlementStatus:   "related_entitlement_status",
	AnonymousDestinationDigest: "anonymous_destination_digest",
	AnonymousDestinationKind:   "anonymous_destination_kind",
	CategoryNo:                 "category_no",
	DepartmentNo:               "department_no",
	AssigneeUserId:             "assignee_user_id",
	Subject:                    "subject",
	BaseStage:                  "base_stage",
	CustomStatusNo:             "custom_status_no",
	PriorityCode:               "priority_code",
	CurrentRound:               "current_round",
	FollowUpRequired:           "follow_up_required",
	SlaRiskCode:                "sla_risk_code",
	ResolvedAt:                 "resolved_at",
	ClosedAt:                   "closed_at",
	ClosedReasonCode:           "closed_reason_code",
	ClosedByType:               "closed_by_type",
	CustomerLastRepliedAt:      "customer_last_replied_at",
	LastActivityAt:             "last_activity_at",
	Version:                    "version",
	CreatedBy:                  "created_by",
	UpdatedBy:                  "updated_by",
	CreatedAt:                  "created_at",
	UpdatedAt:                  "updated_at",
}

// NewSupportTicketDao creates and returns a new DAO object for table data access.
func NewSupportTicketDao(handlers ...gdb.ModelHandler) *SupportTicketDao {
	return &SupportTicketDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket",
		columns:  supportTicketColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketDao) Columns() SupportTicketColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
