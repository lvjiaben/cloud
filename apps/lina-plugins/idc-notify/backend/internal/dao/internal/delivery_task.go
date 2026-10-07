// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryTaskDao is the data access object for the table plugin_idc_notify_delivery_task.
type DeliveryTaskDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  DeliveryTaskColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// DeliveryTaskColumns defines and stores column names for the table plugin_idc_notify_delivery_task.
type DeliveryTaskColumns struct {
	TenantId                    string //
	TaskNo                      string //
	ChainNo                     string //
	PreviousTaskNo              string //
	EventCode                   string //
	TopicCode                   string //
	ChannelCode                 string //
	TemplateNo                  string //
	TemplateVersion             string //
	LocaleCode                  string //
	OwnerUserId                 string //
	CustomerNo                  string //
	ContactNo                   string //
	RecipientMask               string //
	BusinessObjectType          string //
	BusinessObjectNo            string //
	BusinessObjectVersion       string //
	MarketingSnapshotNo         string //
	MarketingDecisionDigest     string //
	MarketingConsentEventNo     string //
	MarketingConsentVersion     string //
	MarketingPreferenceVersion  string //
	MarketingQuietHoursVersion  string //
	MarketingEvaluatedAt        string //
	SubjectContent              string //
	BodyContent                 string //
	ProviderVariables           string //
	ContentDigest               string //
	StatusCode                  string //
	ResultClass                 string //
	ExternalBoundaryCrossed     string //
	ProviderMessageRef          string //
	ProviderQueryDeadline       string //
	ProviderQueryDeadlineSource string //
	RetrySequence               string //
	NextAttemptAt               string //
	AttemptLease                string //
	AttemptLeaseUntil           string //
	Version                     string //
	QueuedAt                    string //
	CompletedAt                 string //
	UpdatedAt                   string //
}

// deliveryTaskColumns holds the columns for the table plugin_idc_notify_delivery_task.
var deliveryTaskColumns = DeliveryTaskColumns{
	TenantId:                    "tenant_id",
	TaskNo:                      "task_no",
	ChainNo:                     "chain_no",
	PreviousTaskNo:              "previous_task_no",
	EventCode:                   "event_code",
	TopicCode:                   "topic_code",
	ChannelCode:                 "channel_code",
	TemplateNo:                  "template_no",
	TemplateVersion:             "template_version",
	LocaleCode:                  "locale_code",
	OwnerUserId:                 "owner_user_id",
	CustomerNo:                  "customer_no",
	ContactNo:                   "contact_no",
	RecipientMask:               "recipient_mask",
	BusinessObjectType:          "business_object_type",
	BusinessObjectNo:            "business_object_no",
	BusinessObjectVersion:       "business_object_version",
	MarketingSnapshotNo:         "marketing_snapshot_no",
	MarketingDecisionDigest:     "marketing_decision_digest",
	MarketingConsentEventNo:     "marketing_consent_event_no",
	MarketingConsentVersion:     "marketing_consent_version",
	MarketingPreferenceVersion:  "marketing_preference_version",
	MarketingQuietHoursVersion:  "marketing_quiet_hours_version",
	MarketingEvaluatedAt:        "marketing_evaluated_at",
	SubjectContent:              "subject_content",
	BodyContent:                 "body_content",
	ProviderVariables:           "provider_variables",
	ContentDigest:               "content_digest",
	StatusCode:                  "status_code",
	ResultClass:                 "result_class",
	ExternalBoundaryCrossed:     "external_boundary_crossed",
	ProviderMessageRef:          "provider_message_ref",
	ProviderQueryDeadline:       "provider_query_deadline",
	ProviderQueryDeadlineSource: "provider_query_deadline_source",
	RetrySequence:               "retry_sequence",
	NextAttemptAt:               "next_attempt_at",
	AttemptLease:                "attempt_lease",
	AttemptLeaseUntil:           "attempt_lease_until",
	Version:                     "version",
	QueuedAt:                    "queued_at",
	CompletedAt:                 "completed_at",
	UpdatedAt:                   "updated_at",
}

// NewDeliveryTaskDao creates and returns a new DAO object for table data access.
func NewDeliveryTaskDao(handlers ...gdb.ModelHandler) *DeliveryTaskDao {
	return &DeliveryTaskDao{
		group:    "default",
		table:    "plugin_idc_notify_delivery_task",
		columns:  deliveryTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *DeliveryTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *DeliveryTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *DeliveryTaskDao) Columns() DeliveryTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *DeliveryTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *DeliveryTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *DeliveryTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
