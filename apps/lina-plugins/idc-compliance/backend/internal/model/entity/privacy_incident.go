// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyIncident is the golang structure for table privacy_incident.
type PrivacyIncident struct {
	Id                              int64      `json:"id"                              orm:"id"                                 description:""`
	TenantId                        int        `json:"tenantId"                        orm:"tenant_id"                          description:""`
	IncidentNo                      string     `json:"incidentNo"                      orm:"incident_no"                        description:""`
	StatusCode                      string     `json:"statusCode"                      orm:"status_code"                        description:""`
	SeverityCode                    string     `json:"severityCode"                    orm:"severity_code"                      description:""`
	SourceCode                      string     `json:"sourceCode"                      orm:"source_code"                        description:""`
	SafeSummary                     string     `json:"safeSummary"                     orm:"safe_summary"                       description:""`
	TicketGeneration                int64      `json:"ticketGeneration"                orm:"ticket_generation"                  description:""`
	ExportBlocked                   bool       `json:"exportBlocked"                   orm:"export_blocked"                     description:""`
	ExternalNotificationConfirmed   bool       `json:"externalNotificationConfirmed"   orm:"external_notification_confirmed"    description:""`
	ExternalNotificationConfirmedBy int        `json:"externalNotificationConfirmedBy" orm:"external_notification_confirmed_by" description:""`
	ExternalNotificationConfirmedAt *time.Time `json:"externalNotificationConfirmedAt" orm:"external_notification_confirmed_at" description:""`
	ResolvedAt                      *time.Time `json:"resolvedAt"                      orm:"resolved_at"                        description:""`
	CreatedByUserId                 int        `json:"createdByUserId"                 orm:"created_by_user_id"                 description:""`
	UpdatedByUserId                 int        `json:"updatedByUserId"                 orm:"updated_by_user_id"                 description:""`
	Version                         int64      `json:"version"                         orm:"version"                            description:""`
	CreatedAt                       *time.Time `json:"createdAt"                       orm:"created_at"                         description:""`
	UpdatedAt                       *time.Time `json:"updatedAt"                       orm:"updated_at"                         description:""`
}
