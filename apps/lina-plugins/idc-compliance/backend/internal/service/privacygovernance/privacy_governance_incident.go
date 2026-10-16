// Incident persistence closes sensitive access and records an explicit response timeline.

package privacygovernance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
)

var incidentStages = []string{
	"discovered", "contained", "assessed", "notification_pending",
	"notification_confirmed", "remediating", "resolved",
}

type incidentRow struct {
	IncidentNo                    string     `json:"incident_no"`
	StatusCode                    string     `json:"status_code"`
	SeverityCode                  string     `json:"severity_code"`
	SourceCode                    string     `json:"source_code"`
	SafeSummary                   string     `json:"safe_summary"`
	TicketGeneration              int64      `json:"ticket_generation"`
	ExportBlocked                 bool       `json:"export_blocked"`
	ExternalNotificationConfirmed bool       `json:"external_notification_confirmed"`
	ResolvedAt                    *time.Time `json:"resolved_at"`
	CreatedAt                     time.Time  `json:"created_at"`
	UpdatedAt                     time.Time  `json:"updated_at"`
	Version                       int64      `json:"version"`
}

type incidentEventRow struct {
	IncidentNo  string    `json:"incident_no"`
	EventNo     string    `json:"event_no"`
	StageCode   string    `json:"stage_code"`
	SafeNote    string    `json:"safe_note"`
	ActorUserID int       `json:"actor_user_id"`
	OccurredAt  time.Time `json:"occurred_at"`
}

func (s *serviceImpl) ListIncidents(ctx context.Context, limit int) ([]Incident, error) {
	if _, err := s.authorize(ctx, PermissionQuery, "privacy-incident-directory"); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var rows []incidentRow
	if err := s.database.Model("plugin_idc_compliance_privacy_incident").Ctx(ctx).
		Fields(incidentFields()).Where("tenant_id", idccap.DefaultTenantID).
		OrderDesc("created_at").OrderDesc("id").Limit(limit).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.projectIncidents(ctx, rows)
}

func (s *serviceImpl) CreateIncident(ctx context.Context, input IncidentCreateInput) (*Incident, error) {
	input.SeverityCode = strings.TrimSpace(input.SeverityCode)
	input.SourceCode = strings.TrimSpace(input.SourceCode)
	input.SafeSummary = strings.TrimSpace(input.SafeSummary)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !oneOf(input.SeverityCode, "low", "medium", "high", "critical") ||
		!oneOf(input.SourceCode, "unauthorized_attempt", "bulk_download", "data_loss", "manual_report", "system_detection") ||
		!validGovernanceText(input.SafeSummary, 2000) || !idempotencyPattern.MatchString(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionIncident, "privacy-incident-directory")
	if err != nil {
		return nil, err
	}
	digest := sha256Hex("privacy-incident-create-key\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex(strings.Join([]string{"privacy-incident-v1", input.SeverityCode, input.SourceCode, input.SafeSummary}, "\x00"))
	now := s.now().UTC().Truncate(time.Microsecond)
	var incidentNo string
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		var replay []struct {
			RequestFingerprint string `json:"request_fingerprint"`
			ResultReference    string `json:"result_reference"`
		}
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT request_fingerprint, result_reference FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = 'create_incident' AND target_no = 'privacy-incident-directory' AND idempotency_digest = ?`, idccap.DefaultTenantID, digest); loadErr != nil {
			return loadErr
		}
		if len(replay) == 1 {
			if replay[0].RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			incidentNo = replay[0].ResultReference
			return nil
		}
		generationValue, loadErr := s.database.GetValue(ctx, `SELECT COALESCE(MAX(ticket_generation), 0) + 1 FROM plugin_idc_compliance_privacy_incident WHERE tenant_id = ?`, idccap.DefaultTenantID)
		if loadErr != nil {
			return loadErr
		}
		incidentNo, err = newBusinessNo("PIN_")
		if err != nil {
			return err
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_incident (tenant_id, incident_no, status_code, severity_code, source_code, safe_summary, ticket_generation, export_blocked, created_by_user_id, updated_by_user_id, version, created_at, updated_at) VALUES (?, ?, 'discovered', ?, ?, ?, ?, TRUE, ?, ?, 1, ?, ?)`, idccap.DefaultTenantID, incidentNo, input.SeverityCode, input.SourceCode, input.SafeSummary, generationValue.Int64(), actor, actor, now, now); insertErr != nil {
			return insertErr
		}
		if eventErr := s.insertIncidentEvent(ctx, incidentNo, "discovered", input.SafeSummary, actor, now); eventErr != nil {
			return eventErr
		}
		actionNo := "PRA_" + sha256Hex("incident-create\x00" + incidentNo + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, 'create_incident', 'privacy-incident-directory', ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, digest, fingerprint, incidentNo, actor); insertErr != nil {
			return insertErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.incident-create", IdempotencyKey: digest,
			OperationCode: "compliance.privacy.incident-create", TargetType: "privacy_incident", TargetNo: incidentNo,
			ResultCode: "succeeded", ReasonCode: "sensitive_access_anomaly_detected", RiskCode: "critical",
			RequiredPermission: PermissionIncident,
			BeforeSnapshot:     map[string]any{"status": "missing"},
			AfterSnapshot:      map[string]any{"status": "discovered", "severity": input.SeverityCode, "source": input.SourceCode, "exports_blocked": true, "access_generation_rotated": true},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.getIncident(ctx, incidentNo)
}

func (s *serviceImpl) AdvanceIncident(ctx context.Context, input IncidentAdvanceInput) (*Incident, error) {
	input.IncidentNo = strings.TrimSpace(input.IncidentNo)
	input.NextStageCode = strings.TrimSpace(input.NextStageCode)
	input.SafeNote = strings.TrimSpace(input.SafeNote)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !businessNoPattern.MatchString(input.IncidentNo) || input.ExpectedVersion <= 0 ||
		!validGovernanceText(input.SafeNote, 2000) || !idempotencyPattern.MatchString(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionIncident, input.IncidentNo)
	if err != nil {
		return nil, err
	}
	digest := sha256Hex("privacy-incident-advance-key\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex(fmt.Sprintf("privacy-incident-advance-v1\x00%s\x00%d\x00%s\x00%t\x00%s", input.IncidentNo, input.ExpectedVersion, input.NextStageCode, input.ConfirmExternalNotification, input.SafeNote))
	now := s.now().UTC().Truncate(time.Microsecond)
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		var rows []incidentRow
		if loadErr := s.database.GetScan(ctx, &rows, `SELECT `+incidentFields()+` FROM plugin_idc_compliance_privacy_incident WHERE tenant_id = ? AND incident_no = ? FOR UPDATE`, idccap.DefaultTenantID, input.IncidentNo); loadErr != nil {
			return loadErr
		}
		if len(rows) != 1 {
			return bizerr.NewCode(CodeNotAccessible)
		}
		row := rows[0]
		var replay []struct {
			RequestFingerprint string `json:"request_fingerprint"`
		}
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT request_fingerprint FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = 'advance_incident' AND target_no = ? AND idempotency_digest = ?`, idccap.DefaultTenantID, input.IncidentNo, digest); loadErr != nil {
			return loadErr
		}
		if len(replay) == 1 {
			if replay[0].RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if row.Version != input.ExpectedVersion || !nextIncidentStage(row.StatusCode, input.NextStageCode) ||
			(input.NextStageCode == "notification_confirmed") != input.ConfirmExternalNotification {
			return bizerr.NewCode(CodeConflict)
		}
		resolvedAt := any(nil)
		exportBlocked := true
		if input.NextStageCode == "resolved" {
			resolvedAt = now
			exportBlocked = false
		}
		notificationConfirmed := input.NextStageCode == "notification_confirmed"
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_incident SET status_code = ?, export_blocked = ?, external_notification_confirmed = CASE WHEN ? THEN TRUE ELSE external_notification_confirmed END, external_notification_confirmed_by = CASE WHEN ? THEN ? ELSE external_notification_confirmed_by END, external_notification_confirmed_at = CASE WHEN ? THEN ? ELSE external_notification_confirmed_at END, resolved_at = ?, updated_by_user_id = ?, version = version + 1, updated_at = ? WHERE tenant_id = ? AND incident_no = ? AND version = ?`, input.NextStageCode, exportBlocked, notificationConfirmed, notificationConfirmed, actor, notificationConfirmed, now, resolvedAt, actor, now, idccap.DefaultTenantID, input.IncidentNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if eventErr := s.insertIncidentEvent(ctx, input.IncidentNo, input.NextStageCode, input.SafeNote, actor, now); eventErr != nil {
			return eventErr
		}
		actionNo := "PRA_" + sha256Hex("incident-advance\x00" + input.IncidentNo + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, 'advance_incident', ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, input.IncidentNo, digest, fingerprint, input.NextStageCode, actor); insertErr != nil {
			return insertErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.incident-advance", IdempotencyKey: digest,
			OperationCode: "compliance.privacy.incident-advance", TargetType: "privacy_incident", TargetNo: input.IncidentNo,
			ResultCode: "succeeded", ReasonCode: "privacy_incident_stage_advanced", RiskCode: "critical",
			RequiredPermission: PermissionIncident,
			BeforeSnapshot:     map[string]any{"status": row.StatusCode, "version": row.Version},
			AfterSnapshot:      map[string]any{"status": input.NextStageCode, "external_notification_confirmed": notificationConfirmed, "exports_blocked": exportBlocked},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.getIncident(ctx, input.IncidentNo)
}

// CurrentSensitiveAccessGeneration denies issuance and opening while any event is unresolved.
func (s *serviceImpl) CurrentSensitiveAccessGeneration(ctx context.Context) (int64, bool, error) {
	var rows []struct {
		Generation int64 `json:"generation"`
		Blocked    bool  `json:"blocked"`
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT COALESCE(MAX(ticket_generation), 0) + 1 AS generation, COALESCE(bool_or(status_code <> 'resolved'), FALSE) AS blocked FROM plugin_idc_compliance_privacy_incident WHERE tenant_id = ?`, idccap.DefaultTenantID); err != nil || len(rows) != 1 {
		return 0, true, bizerr.NewCode(CodeUnavailable)
	}
	return rows[0].Generation, rows[0].Blocked, nil
}

func (s *serviceImpl) GenerateSupportExport(ctx context.Context, input SupportExportInput) (*SupportExport, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.SupportAgreementConfirmed || !idempotencyPattern.MatchString(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionConfigure, "privacy-support-export")
	if err != nil {
		return nil, err
	}
	if _, blocked, gateErr := s.CurrentSensitiveAccessGeneration(ctx); gateErr != nil || blocked {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	assessment, err := s.CurrentPublished(ctx, AssessmentLocalSensitiveExport)
	if err != nil {
		return nil, err
	}
	collection, err := s.RequireCollection(ctx, AssessmentLocalSensitiveExport, DataLocalSensitiveExport)
	if err != nil {
		return nil, err
	}
	var statusRows []struct {
		Area   string `json:"area"`
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	if err = s.database.GetScan(ctx, &statusRows, `SELECT 'incident' AS area, status_code AS status, COUNT(*) AS count FROM plugin_idc_compliance_privacy_incident WHERE tenant_id = ? GROUP BY status_code UNION ALL SELECT 'deletion_task' AS area, status_code AS status, COUNT(*) AS count FROM plugin_idc_compliance_privacy_deletion_task WHERE tenant_id = ? GROUP BY status_code ORDER BY area, status`, idccap.DefaultTenantID, idccap.DefaultTenantID); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	payload := struct {
		SchemaVersion string `json:"schemaVersion"`
		Assessment    struct {
			Number  string `json:"number"`
			Version int64  `json:"version"`
		} `json:"assessment"`
		StatusCounts any `json:"statusCounts"`
	}{SchemaVersion: "idc-redacted-support-v1", StatusCounts: statusRows}
	payload.Assessment.Number = assessment.AssessmentNo
	payload.Assessment.Version = assessment.Version
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	digest := sha256Hex(string(content))
	idempotencyDigest := sha256Hex("privacy-support-export-key\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex("privacy-support-export-v1\x00support-agreement-confirmed")
	exportNo := ""
	now := s.now().UTC().Truncate(time.Microsecond)
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		var replay []struct {
			RequestFingerprint string `json:"request_fingerprint"`
			ResultReference    string `json:"result_reference"`
		}
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT request_fingerprint, result_reference FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = 'generate_support_export' AND target_no = 'privacy-support-export' AND idempotency_digest = ?`, idccap.DefaultTenantID, idempotencyDigest); loadErr != nil {
			return loadErr
		}
		if len(replay) == 1 {
			if replay[0].RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			exportNo = replay[0].ResultReference
			return nil
		}
		exportNo = "PSE_" + sha256Hex("support-export\x00" + input.IdempotencyKey)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_support_export (tenant_id, export_no, purpose_code, assessment_no, field_count, record_count, content_sha256, content_bytes, status_code, retention_until, created_by_user_id, created_at) VALUES (?, ?, 'support_diagnosis', ?, 3, ?, ?, ?, 'active', ?, ?, ?)`, idccap.DefaultTenantID, exportNo, assessment.AssessmentNo, len(statusRows), digest, content, collection.RetentionUntil, actor, now); insertErr != nil {
			return insertErr
		}
		actionNo := "PRA_" + sha256Hex("support-export\x00" + exportNo + "\x00" + idempotencyDigest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, 'generate_support_export', 'privacy-support-export', ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, idempotencyDigest, fingerprint, exportNo, actor); insertErr != nil {
			return insertErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"export_no": ""},
			IdempotencyScope: "compliance.privacy.support-export", IdempotencyKey: idempotencyDigest,
			OperationCode: "compliance.privacy.support-export", TargetType: "local_sensitive_export", TargetNo: exportNo,
			ResultCode: "succeeded", ReasonCode: "separate_support_agreement_confirmed", RiskCode: "high",
			RequiredPermission: PermissionConfigure,
			AfterSnapshot:      map[string]any{"redacted": true, "field_count": 3, "record_count": len(statusRows), "remote_access_opened": false},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.loadSupportExport(ctx, exportNo)
}

func (s *serviceImpl) loadSupportExport(ctx context.Context, exportNo string) (*SupportExport, error) {
	var rows []struct {
		ExportNo      string    `json:"export_no"`
		Content       []byte    `json:"content_bytes"`
		ContentSHA256 string    `json:"content_sha256"`
		FieldCount    int       `json:"field_count"`
		RecordCount   int       `json:"record_count"`
		CreatedAt     time.Time `json:"created_at"`
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT export_no, content_bytes, content_sha256, field_count, record_count, created_at FROM plugin_idc_compliance_privacy_support_export WHERE tenant_id = ? AND export_no = ? AND status_code = 'active' LIMIT 1`, idccap.DefaultTenantID, exportNo); err != nil || len(rows) != 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	row := rows[0]
	if sha256Hex(string(row.Content)) != row.ContentSHA256 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &SupportExport{ExportNo: row.ExportNo, FileName: row.ExportNo + ".json", Content: row.Content, ContentSHA256: row.ContentSHA256, FieldCount: row.FieldCount, RecordCount: row.RecordCount, CreatedAt: row.CreatedAt.UTC()}, nil
}

func (s *serviceImpl) ProjectAISafeFields(ctx context.Context, input map[string]any) (map[string]any, error) {
	if _, err := s.RequireCollection(ctx, AssessmentAISafeProjection, DataSensitiveAccessAudit); err != nil {
		return nil, err
	}
	assessment, err := s.CurrentPublished(ctx, AssessmentAISafeProjection)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(assessment.AIAllowedFields))
	for _, field := range assessment.AIAllowedFields {
		allowed[field] = struct{}{}
	}
	output := make(map[string]any, len(input))
	for field, value := range input {
		if _, cataloged := aiFieldCatalog[field]; !cataloged {
			return nil, bizerr.NewCode(CodeNotAccessible)
		}
		if _, enabled := allowed[field]; !enabled || !safeAIValue(field, value) {
			return nil, bizerr.NewCode(CodeNotAccessible)
		}
		output[field] = value
	}
	return output, nil
}

func safeAIValue(field string, value any) bool {
	if strings.HasPrefix(field, "aggregate_") {
		switch number := value.(type) {
		case int:
			return number >= 0
		case int64:
			return number >= 0
		default:
			return false
		}
	}
	text, ok := value.(string)
	return ok && len(text) > 0 && len(text) <= 32 && !strings.ContainsAny(text, "@/:\\")
}

func (s *serviceImpl) getIncident(ctx context.Context, incidentNo string) (*Incident, error) {
	var rows []incidentRow
	if err := s.database.GetScan(ctx, &rows, `SELECT `+incidentFields()+` FROM plugin_idc_compliance_privacy_incident WHERE tenant_id = ? AND incident_no = ? LIMIT 1`, idccap.DefaultTenantID, incidentNo); err != nil || len(rows) != 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	items, err := s.projectIncidents(ctx, rows)
	if err != nil || len(items) != 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &items[0], nil
}

func (s *serviceImpl) projectIncidents(ctx context.Context, rows []incidentRow) ([]Incident, error) {
	items := make([]Incident, 0, len(rows))
	if len(rows) == 0 {
		return items, nil
	}
	numbers := make([]string, 0, len(rows))
	for _, row := range rows {
		numbers = append(numbers, row.IncidentNo)
	}
	var events []incidentEventRow
	if err := s.database.Model("plugin_idc_compliance_privacy_incident_event").Ctx(ctx).
		Fields("incident_no, event_no, stage_code, safe_note, actor_user_id, occurred_at").
		Where("tenant_id", idccap.DefaultTenantID).WhereIn("incident_no", numbers).
		OrderAsc("occurred_at").OrderAsc("id").Scan(&events); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	byIncident := make(map[string][]IncidentEvent, len(rows))
	for _, event := range events {
		byIncident[event.IncidentNo] = append(byIncident[event.IncidentNo], IncidentEvent{EventNo: event.EventNo, StageCode: event.StageCode, SafeNote: event.SafeNote, ActorUserID: event.ActorUserID, OccurredAt: event.OccurredAt.UTC()})
	}
	for _, row := range rows {
		items = append(items, Incident{IncidentNo: row.IncidentNo, StatusCode: row.StatusCode, SeverityCode: row.SeverityCode, SourceCode: row.SourceCode, SafeSummary: row.SafeSummary, TicketGeneration: row.TicketGeneration, ExportBlocked: row.ExportBlocked, ExternalNotificationConfirmed: row.ExternalNotificationConfirmed, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), ResolvedAt: row.ResolvedAt, Version: row.Version, Timeline: byIncident[row.IncidentNo]})
	}
	return items, nil
}

func (s *serviceImpl) insertIncidentEvent(ctx context.Context, incidentNo, stage, note string, actor int, now time.Time) error {
	eventNo, err := newBusinessNo("PIE_")
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_incident_event (tenant_id, event_no, incident_no, stage_code, safe_note, actor_user_id, occurred_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, eventNo, incidentNo, stage, note, actor, now, now)
	return err
}

func nextIncidentStage(current, next string) bool {
	for index := 0; index+1 < len(incidentStages); index++ {
		if incidentStages[index] == current && incidentStages[index+1] == next {
			return true
		}
	}
	return false
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func incidentFields() string {
	return `incident_no, status_code, severity_code, source_code, safe_summary, ticket_generation, export_blocked, external_notification_confirmed, resolved_at, created_at, updated_at, version`
}
