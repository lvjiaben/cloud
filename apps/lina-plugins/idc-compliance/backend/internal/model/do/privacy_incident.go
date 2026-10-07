// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyIncident is the golang structure of table plugin_idc_compliance_privacy_incident for DAO operations like Where/Data.
type PrivacyIncident struct {
	g.Meta                          `orm:"table:plugin_idc_compliance_privacy_incident, do:true"`
	Id                              any        //
	TenantId                        any        //
	IncidentNo                      any        //
	StatusCode                      any        //
	SeverityCode                    any        //
	SourceCode                      any        //
	SafeSummary                     any        //
	TicketGeneration                any        //
	ExportBlocked                   any        //
	ExternalNotificationConfirmed   any        //
	ExternalNotificationConfirmedBy any        //
	ExternalNotificationConfirmedAt *time.Time //
	ResolvedAt                      *time.Time //
	CreatedByUserId                 any        //
	UpdatedByUserId                 any        //
	Version                         any        //
	CreatedAt                       *time.Time //
	UpdatedAt                       *time.Time //
}
