// Privacy incident API types expose safe response state and timeline events. They never carry affected private records or external credentials.

package v1

type PrivacyIncidentEventItem struct {
	EventNo     string `json:"eventNo" dc:"Opaque incident event number" eg:"PIE_0123456789abcdef0123456789abcdef"`
	StageCode   string `json:"stageCode" dc:"Response stage: discovered, contained, assessed, notification_pending, notification_confirmed, remediating, or resolved" eg:"contained"`
	SafeNote    string `json:"safeNote" dc:"Bounded event note that must not contain private incident material" eg:"Affected export access has been disabled"`
	ActorUserID int    `json:"actorUserId" dc:"Administrator identifier that recorded the event" eg:"1"`
	OccurredAt  int64  `json:"occurredAt" dc:"Event time as a Unix timestamp in milliseconds" eg:"1787040000000"`
}

type PrivacyIncidentItem struct {
	IncidentNo                    string                     `json:"incidentNo" dc:"Opaque privacy incident number" eg:"PIN_0123456789abcdef0123456789abcdef"`
	StatusCode                    string                     `json:"statusCode" dc:"Current response stage: discovered, contained, assessed, notification_pending, notification_confirmed, remediating, or resolved" eg:"discovered"`
	SeverityCode                  string                     `json:"severityCode" dc:"Severity: low, medium, high, or critical" eg:"high"`
	SourceCode                    string                     `json:"sourceCode" dc:"Detection source: unauthorized_attempt, bulk_download, data_loss, manual_report, or system_detection" eg:"system_detection"`
	SafeSummary                   string                     `json:"safeSummary" dc:"Bounded summary without raw affected records or credentials" eg:"An unusual sensitive export attempt was detected"`
	TicketGeneration              int64                      `json:"ticketGeneration" dc:"Sensitive-access ticket generation revoked by this incident" eg:"2"`
	ExportBlocked                 bool                       `json:"exportBlocked" dc:"Whether local sensitive export is currently blocked" eg:"true"`
	ExternalNotificationConfirmed bool                       `json:"externalNotificationConfirmed" dc:"Whether an administrator explicitly confirmed external notification" eg:"false"`
	CreatedAt                     int64                      `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1787040000000"`
	UpdatedAt                     int64                      `json:"updatedAt" dc:"Last update time as a Unix timestamp in milliseconds" eg:"1787040000000"`
	ResolvedAt                    int64                      `json:"resolvedAt" dc:"Resolution time as a Unix timestamp in milliseconds; zero until resolved" eg:"0"`
	Version                       int64                      `json:"version" dc:"Current optimistic concurrency version" eg:"1"`
	Timeline                      []PrivacyIncidentEventItem `json:"timeline" dc:"Ordered safe local response timeline" eg:"[]"`
}
