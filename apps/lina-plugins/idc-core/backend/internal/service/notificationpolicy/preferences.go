package notificationpolicy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

var (
	CodePreferenceInvalid       = bizerr.MustDefine("IDC_NOTIFICATION_PREFERENCE_INVALID", "The notification preference request is invalid", gcode.CodeInvalidParameter)
	CodePreferenceNotAccessible = bizerr.MustDefine("IDC_NOTIFICATION_PREFERENCE_NOT_ACCESSIBLE", "The notification preference is not accessible", gcode.CodeNotAuthorized)
	CodePreferenceConflict      = bizerr.MustDefine("IDC_NOTIFICATION_PREFERENCE_CONFLICT", "The notification preference changed; reload it and try again", gcode.CodeInvalidOperation)
	CodePreferenceUnavailable   = bizerr.MustDefine("IDC_NOTIFICATION_PREFERENCE_UNAVAILABLE", "Notification preferences are temporarily unavailable", gcode.CodeInternalError)
)

type preferenceCommander struct {
	database  gdb.DB
	bizCtx    bizctxcap.Service
	principal principal.Reader
	random    io.Reader
}

var _ notificationcap.PreferenceService = (*preferenceCommander)(nil)

func NewPreferenceCommander(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader) (notificationcap.PreferenceService, error) {
	if database == nil || bizCtx == nil || principalReader == nil {
		return nil, gerror.New("notification preference commander requires database and current identity")
	}
	return &preferenceCommander{database: database, bizCtx: bizCtx, principal: principalReader, random: rand.Reader}, nil
}

func (s *preferenceCommander) Current(ctx context.Context) (*notificationcap.PreferenceSnapshot, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	var head []struct {
		PreferredLocale string `orm:"preferred_locale"`
		Version         int64  `orm:"version"`
	}
	if err = s.database.GetScan(ctx, &head, `SELECT c.preferred_locale,COALESCE(p.version,0) AS version FROM plugin_idc_core_customer_subject c LEFT JOIN plugin_idc_core_notification_preference_profile p ON p.tenant_id=0 AND p.customer_no=c.customer_no WHERE c.tenant_id=0 AND c.customer_no=? AND c.status_code='active' LIMIT 1`, customerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodePreferenceUnavailable)
	}
	if len(head) != 1 {
		return nil, bizerr.NewCode(CodePreferenceNotAccessible)
	}
	var preferences []notificationcap.PreferenceProjection
	if err = s.database.GetScan(ctx, &preferences, `SELECT contact_no,topic_code AS topic,channel_code AS channel,enabled,version FROM plugin_idc_core_customer_contact_preference WHERE tenant_id=0 AND customer_no=? ORDER BY contact_no,topic_code,channel_code LIMIT ?`, customerNo, notificationcap.MaximumPreferenceItems+1); err != nil {
		return nil, bizerr.WrapCode(err, CodePreferenceUnavailable)
	}
	if len(preferences) > notificationcap.MaximumPreferenceItems {
		return nil, bizerr.NewCode(CodePreferenceUnavailable)
	}
	var consents []notificationcap.MarketingConsentProjection
	if err = s.database.GetScan(ctx, &consents, `SELECT c.contact_no,c.channel_code AS channel,c.status_code AS status,e.policy_reference,e.policy_version,e.locale,c.version FROM plugin_idc_core_marketing_channel_consent c INNER JOIN plugin_idc_core_marketing_channel_consent_event e ON e.tenant_id=c.tenant_id AND e.event_no=c.current_event_no AND e.customer_no=c.customer_no AND e.contact_no=c.contact_no AND e.channel_code=c.channel_code WHERE c.tenant_id=0 AND c.customer_no=? ORDER BY c.contact_no,c.channel_code LIMIT ?`, customerNo, notificationcap.MaximumPreferenceItems+1); err != nil {
		return nil, bizerr.WrapCode(err, CodePreferenceUnavailable)
	}
	if len(consents) > notificationcap.MaximumPreferenceItems {
		return nil, bizerr.NewCode(CodePreferenceUnavailable)
	}
	var quietRows []struct {
		TimezoneName string `orm:"timezone_name"`
		StartMinute  int    `orm:"start_minute"`
		EndMinute    int    `orm:"end_minute"`
		Enabled      bool   `orm:"enabled"`
		Version      int64  `orm:"version"`
	}
	if err = s.database.GetScan(ctx, &quietRows, `SELECT timezone_name,start_minute,end_minute,enabled,version FROM plugin_idc_core_marketing_quiet_hours WHERE tenant_id=0 AND customer_no=? LIMIT 1`, customerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodePreferenceUnavailable)
	}
	result := &notificationcap.PreferenceSnapshot{
		CustomerNo:        customerNo,
		PreferredLocale:   head[0].PreferredLocale,
		Version:           head[0].Version,
		Preferences:       preferences,
		MarketingConsents: consents,
	}
	if result.Preferences == nil {
		result.Preferences = make([]notificationcap.PreferenceProjection, 0)
	}
	if result.MarketingConsents == nil {
		result.MarketingConsents = make([]notificationcap.MarketingConsentProjection, 0)
	}
	if len(quietRows) == 1 {
		quiet := quietRows[0]
		result.QuietHours = &notificationcap.QuietHoursProjection{
			TimezoneName: quiet.TimezoneName, StartMinute: quiet.StartMinute,
			EndMinute: quiet.EndMinute, CrossesMidnight: quiet.Enabled && quiet.StartMinute > quiet.EndMinute,
			Enabled: quiet.Enabled, Version: quiet.Version,
		}
	}
	return result, nil
}

type normalizedPreferenceCommand struct {
	notificationcap.PreferenceCommand
	Fingerprint, IdempotencyDigest, EvidenceDigest string
}

func (s *preferenceCommander) UpdateCurrent(ctx context.Context, input notificationcap.PreferenceCommand) (*notificationcap.PreferenceResult, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	command, err := normalizePreferenceCommand(input)
	if err != nil {
		return nil, err
	}
	var result *notificationcap.PreferenceResult
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var replay []struct {
			RequestFingerprint, EvidenceDigest string
			ResultVersion                      int64
			AppliedAt                          time.Time
		}
		if err := s.database.GetScan(ctx, &replay, `SELECT request_fingerprint,evidence_digest,result_version,applied_at FROM plugin_idc_core_notification_preference_command WHERE tenant_id=0 AND customer_no=? AND idempotency_digest=? LIMIT 1`, customerNo, command.IdempotencyDigest); err != nil {
			return err
		}
		if len(replay) == 1 {
			if replay[0].RequestFingerprint != command.Fingerprint {
				return bizerr.NewCode(CodePreferenceConflict)
			}
			result = &notificationcap.PreferenceResult{CustomerNo: customerNo, Version: replay[0].ResultVersion, EvidenceDigest: replay[0].EvidenceDigest, AppliedAt: replay[0].AppliedAt, Reused: true}
			return nil
		}
		var versions []struct{ Version int64 }
		if err := s.database.GetScan(ctx, &versions, `SELECT version FROM plugin_idc_core_notification_preference_profile WHERE tenant_id=0 AND customer_no=? FOR UPDATE`, customerNo); err != nil {
			return err
		}
		currentVersion := int64(0)
		if len(versions) == 1 {
			currentVersion = versions[0].Version
		}
		if currentVersion != command.ExpectedVersion {
			return bizerr.NewCode(CodePreferenceConflict)
		}
		if err := s.validatePreferenceContacts(ctx, customerNo, command); err != nil {
			return err
		}
		nowValue, err := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if err != nil {
			return err
		}
		now := nowValue.Time()
		for _, change := range command.Changes {
			if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact_preference (tenant_id,customer_no,contact_no,topic_code,channel_code,enabled) VALUES (0,?,?,?,?,?) ON CONFLICT (contact_no,topic_code,channel_code) DO UPDATE SET enabled=EXCLUDED.enabled,version=plugin_idc_core_customer_contact_preference.version+1,updated_at=CURRENT_TIMESTAMP`, customerNo, change.ContactNo, string(change.Topic), string(change.Channel), change.Enabled); err != nil {
				return err
			}
		}
		if command.MarketingConsent != nil {
			if err = s.applyMarketingConsent(ctx, customerNo, command, now); err != nil {
				return err
			}
		}
		if command.QuietHours != nil {
			q := command.QuietHours
			if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_quiet_hours (tenant_id,customer_no,timezone_name,start_minute,end_minute,enabled) VALUES (0,?,?,?,?,?) ON CONFLICT (customer_no) DO UPDATE SET timezone_name=EXCLUDED.timezone_name,start_minute=EXCLUDED.start_minute,end_minute=EXCLUDED.end_minute,enabled=EXCLUDED.enabled,version=plugin_idc_core_marketing_quiet_hours.version+1,updated_at=CURRENT_TIMESTAMP`, customerNo, q.TimezoneName, q.StartMinute, q.EndMinute, q.Enabled); err != nil {
				return err
			}
		}
		nextVersion := currentVersion + 1
		if currentVersion == 0 {
			if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_notification_preference_profile (tenant_id,customer_no,version,updated_at) VALUES (0,?,1,?)`, customerNo, now); err != nil {
				return err
			}
		} else {
			update, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_notification_preference_profile SET version=version+1,updated_at=? WHERE tenant_id=0 AND customer_no=? AND version=?`, now, customerNo, currentVersion)
			if err != nil {
				return err
			}
			changed, _ := update.RowsAffected()
			if changed != 1 {
				return bizerr.NewCode(CodePreferenceConflict)
			}
		}
		commandNo, err := preferenceNumber(s.random, "NPC_")
		if err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_notification_preference_command (tenant_id,command_no,customer_no,idempotency_digest,request_fingerprint,evidence_digest,result_version,applied_at) VALUES (0,?,?,?,?,?,?,?)`, commandNo, customerNo, command.IdempotencyDigest, command.Fingerprint, command.EvidenceDigest, nextVersion, now); err != nil {
			return err
		}
		result = &notificationcap.PreferenceResult{CustomerNo: customerNo, Version: nextVersion, EvidenceDigest: command.EvidenceDigest, AppliedAt: now}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodePreferenceUnavailable)
	}
	return result, nil
}

func (s *preferenceCommander) currentCustomer(ctx context.Context) (string, error) {
	current := s.bizCtx.Current(ctx)
	directCustomer := current.ActingUserID == 0 && !current.IsImpersonation && !current.ActingAsTenant
	delegatedCustomer := current.IsImpersonation && current.ActingUserID > 0 && current.ActingUserID != current.UserID && !current.ActingAsTenant
	if current.UserID <= 0 || current.TenantID != 0 || (!directCustomer && !delegatedCustomer) {
		return "", bizerr.WrapCode(gerror.New("current notification preference context is invalid"), CodePreferenceNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil {
		return "", bizerr.WrapCode(err, CodePreferenceNotAccessible)
	}
	if audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return "", bizerr.WrapCode(gerror.New("current notification preference audience binding is invalid"), CodePreferenceNotAccessible)
	}
	if !notificationIdentifierPattern.MatchString(audience.CustomerNo) {
		return "", bizerr.WrapCode(gerror.New("current notification preference customer number is invalid"), CodePreferenceNotAccessible)
	}
	return audience.CustomerNo, nil
}
func normalizePreferenceCommand(input notificationcap.PreferenceCommand) (normalizedPreferenceCommand, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 128 || input.ExpectedVersion < 0 || len(input.Changes) > 30 || (len(input.Changes) == 0 && input.MarketingConsent == nil && input.QuietHours == nil) {
		return normalizedPreferenceCommand{}, bizerr.NewCode(CodePreferenceInvalid)
	}
	if err := normalizePreferenceChanges(&input); err != nil {
		return normalizedPreferenceCommand{}, err
	}
	if err := normalizeMarketingConsent(input.MarketingConsent); err != nil {
		return normalizedPreferenceCommand{}, err
	}
	if err := normalizeQuietHours(input.QuietHours); err != nil {
		return normalizedPreferenceCommand{}, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return normalizedPreferenceCommand{}, bizerr.NewCode(CodePreferenceInvalid)
	}
	fingerprint := sha256HexPreference(raw)
	idem := sha256HexPreference([]byte("notification-preference-idempotency-v1\x00" + input.IdempotencyKey))
	evidence := sha256HexPreference(append([]byte("notification-preference-evidence-v1\x00"), raw...))
	return normalizedPreferenceCommand{PreferenceCommand: input, Fingerprint: fingerprint, IdempotencyDigest: idem, EvidenceDigest: evidence}, nil
}

func normalizePreferenceChanges(input *notificationcap.PreferenceCommand) error {
	seen := make(map[string]struct{}, len(input.Changes))
	for i := range input.Changes {
		change := &input.Changes[i]
		change.ContactNo = strings.TrimSpace(change.ContactNo)
		if !notificationIdentifierPattern.MatchString(change.ContactNo) || !validTopic(change.Topic) || !validChannel(change.Channel) || (change.Topic == notificationcap.TopicSecurity && !change.Enabled) {
			return bizerr.NewCode(CodePreferenceInvalid)
		}
		key := change.ContactNo + "\x00" + string(change.Topic) + "\x00" + string(change.Channel)
		if _, ok := seen[key]; ok {
			return bizerr.NewCode(CodePreferenceInvalid)
		}
		seen[key] = struct{}{}
	}
	sort.Slice(input.Changes, func(i, j int) bool {
		left := input.Changes[i].ContactNo + string(input.Changes[i].Topic) + string(input.Changes[i].Channel)
		right := input.Changes[j].ContactNo + string(input.Changes[j].Topic) + string(input.Changes[j].Channel)
		return left < right
	})
	return nil
}

func normalizeMarketingConsent(consent *notificationcap.MarketingConsentChange) error {
	if consent == nil {
		return nil
	}
	consent.ContactNo = strings.TrimSpace(consent.ContactNo)
	consent.Action = strings.TrimSpace(consent.Action)
	consent.PolicyReference = strings.TrimSpace(consent.PolicyReference)
	consent.PolicyVersion = strings.TrimSpace(consent.PolicyVersion)
	consent.Locale = strings.TrimSpace(consent.Locale)
	consent.EvidenceDigest = strings.TrimSpace(consent.EvidenceDigest)
	if !notificationIdentifierPattern.MatchString(consent.ContactNo) || !validChannel(consent.Channel) || (consent.Action != "granted" && consent.Action != "withdrawn") || len(consent.PolicyReference) < 1 || len(consent.PolicyReference) > 256 || len(consent.PolicyVersion) < 1 || len(consent.PolicyVersion) > 64 || (consent.Locale != "zh-CN" && consent.Locale != "en-US") || !lowerHex64(consent.EvidenceDigest) {
		return bizerr.NewCode(CodePreferenceInvalid)
	}
	return nil
}

func normalizeQuietHours(quiet *notificationcap.QuietHoursChange) error {
	if quiet == nil {
		return nil
	}
	quiet.TimezoneName = strings.TrimSpace(quiet.TimezoneName)
	if len(quiet.TimezoneName) < 1 || len(quiet.TimezoneName) > 64 || quiet.StartMinute < 0 || quiet.StartMinute > 1439 || quiet.EndMinute < 0 || quiet.EndMinute > 1439 || (quiet.Enabled && quiet.StartMinute == quiet.EndMinute) {
		return bizerr.NewCode(CodePreferenceInvalid)
	}
	if _, err := time.LoadLocation(quiet.TimezoneName); err != nil {
		return bizerr.NewCode(CodePreferenceInvalid)
	}
	return nil
}
func (s *preferenceCommander) validatePreferenceContacts(ctx context.Context, customerNo string, command normalizedPreferenceCommand) error {
	contacts := make(map[string]struct{})
	for _, change := range command.Changes {
		contacts[change.ContactNo] = struct{}{}
	}
	if command.MarketingConsent != nil {
		contacts[command.MarketingConsent.ContactNo] = struct{}{}
	}
	if len(contacts) == 0 {
		return nil
	}
	args := make([]any, 0, len(contacts)+1)
	args = append(args, customerNo)
	values := make([]string, 0, len(contacts))
	for contact := range contacts {
		values = append(values, contact)
	}
	sort.Strings(values)
	for _, contact := range values {
		args = append(args, contact)
	}
	var count int
	if err := s.database.GetScan(ctx, &count, `SELECT COUNT(1) FROM plugin_idc_core_customer_contact WHERE tenant_id=0 AND customer_no=? AND contact_no IN (`+preferencePlaceholders(len(values))+`) AND status_code='active'`, args...); err != nil {
		return err
	}
	if count != len(values) {
		return bizerr.NewCode(CodePreferenceNotAccessible)
	}
	return nil
}
func (s *preferenceCommander) applyMarketingConsent(ctx context.Context, customerNo string, command normalizedPreferenceCommand, now time.Time) error {
	consent := command.MarketingConsent
	eventNo, err := preferenceNumber(s.random, "MCE_")
	if err != nil {
		return err
	}
	eventFingerprint := sha256HexPreference([]byte(command.Fingerprint + "\x00marketing"))
	idem := sha256HexPreference([]byte(command.IdempotencyDigest + "\x00marketing"))
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_channel_consent_event (tenant_id,event_no,customer_no,contact_no,channel_code,action_code,policy_reference,policy_version,locale,source_code,evidence_digest,idempotency_key_digest,request_fingerprint,occurred_at) VALUES (0,?,?,?,?,?,?,?,?,?,'notify',?,?,?,?)`, eventNo, customerNo, consent.ContactNo, string(consent.Channel), consent.Action, consent.PolicyReference, consent.PolicyVersion, consent.Locale, consent.EvidenceDigest, idem, eventFingerprint, now); err != nil {
		return err
	}
	status := "opted_out"
	enabled := false
	if consent.Action == "granted" {
		status = "opted_in"
		enabled = true
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_channel_consent (tenant_id,customer_no,contact_no,channel_code,status_code,current_event_no,consent_occurred_at) VALUES (0,?,?,?,?,?,?) ON CONFLICT (customer_no,contact_no,channel_code) DO UPDATE SET status_code=EXCLUDED.status_code,current_event_no=EXCLUDED.current_event_no,consent_occurred_at=EXCLUDED.consent_occurred_at,version=plugin_idc_core_marketing_channel_consent.version+1,updated_at=CURRENT_TIMESTAMP`, customerNo, consent.ContactNo, string(consent.Channel), status, eventNo, now); err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact_preference (tenant_id,customer_no,contact_no,topic_code,channel_code,enabled) VALUES (0,?,?,'marketing',?,?) ON CONFLICT (contact_no,topic_code,channel_code) DO UPDATE SET enabled=EXCLUDED.enabled,version=plugin_idc_core_customer_contact_preference.version+1,updated_at=CURRENT_TIMESTAMP`, customerNo, consent.ContactNo, string(consent.Channel), enabled)
	return err
}
func lowerHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
func sha256HexPreference(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func preferenceNumber(random io.Reader, prefix string) (string, error) {
	buffer := make([]byte, 16)
	if _, err := io.ReadFull(random, buffer); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buffer), nil
}
func preferencePlaceholders(count int) string {
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}
