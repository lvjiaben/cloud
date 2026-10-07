// This file owns fail-closed marketing consent and pre-send decisions.

package marketing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumMarketingGateBatch = 500

const (
	MarketingConsentGranted   = "granted"
	MarketingConsentWithdrawn = "withdrawn"

	MarketingGateAllowed                = "allowed"
	MarketingGateCampaignUnavailable    = "campaign_unavailable"
	MarketingGateSnapshotUnavailable    = "snapshot_unavailable"
	MarketingGateOutsideAudience        = "outside_audience"
	MarketingGateCustomerUnavailable    = "customer_unavailable"
	MarketingGateContactUnavailable     = "contact_unavailable"
	MarketingGateDestinationUnavailable = "destination_unavailable"
	MarketingGateConsentMissing         = "consent_missing"
	MarketingGateConsentWithdrawn       = "consent_withdrawn"
	MarketingGatePreferenceDisabled     = "preference_disabled"
	MarketingGateQuietHours             = "quiet_hours"
)

// MarketingConsentInput appends one explicit per-contact channel decision.
type MarketingConsentInput struct {
	CustomerNo      string
	ContactNo       string
	Channel         string
	Action          string
	PolicyReference string
	PolicyVersion   string
	Locale          string
	Source          string
	EvidenceDigest  string
	IdempotencyKey  string
}

// MarketingConsentResult is the current projection and immutable event reference.
type MarketingConsentResult struct {
	EventNo    string
	CustomerNo string
	ContactNo  string
	Channel    string
	Status     string
	Version    int64
	OccurredAt time.Time
	Reused     bool
}

// MarketingQuietHoursInput replaces one customer-local quiet-hours projection.
type MarketingQuietHoursInput struct {
	CustomerNo      string
	TimezoneName    string
	StartMinute     int
	EndMinute       int
	Enabled         bool
	ExpectedVersion int64
}

// MarketingQuietHours is a versioned customer-local schedule.
type MarketingQuietHours struct {
	CustomerNo   string
	TimezoneName string
	StartMinute  int
	EndMinute    int
	Enabled      bool
	Version      int64
}

// MarketingSendGateInput binds one content version to one frozen campaign target.
type MarketingSendGateInput struct {
	CampaignNo    string
	SnapshotNo    string
	CustomerNo    string
	ContactNo     string
	Channel       string
	ContentDigest string
}

// MarketingSendDecision is evidence for notify-owned delivery creation.
type MarketingSendDecision struct {
	CampaignNo        string
	SnapshotNo        string
	CustomerNo        string
	ContactNo         string
	Channel           string
	Allowed           bool
	ReasonCode        string
	ConsentEventNo    string
	ConsentVersion    int64
	PreferenceVersion int64
	QuietHoursVersion int64
	EvaluatedAt       time.Time
	DecisionDigest    string
}

// EngagementService owns marketing permission facts but never sends notifications.
type EngagementService interface {
	SetConsent(ctx context.Context, input MarketingConsentInput) (*MarketingConsentResult, error)
	SetQuietHours(ctx context.Context, input MarketingQuietHoursInput) (*MarketingQuietHours, error)
	CheckSendBatch(ctx context.Context, inputs []MarketingSendGateInput) ([]MarketingSendDecision, error)
}

type engagementServiceImpl struct {
	database gdb.DB
	random   io.Reader
}

var _ EngagementService = (*engagementServiceImpl)(nil)

// NewEngagementService constructs a local marketing governance service.
func NewEngagementService(database gdb.DB) (EngagementService, error) {
	return newEngagementService(database, rand.Reader)
}

func newEngagementService(database gdb.DB, random io.Reader) (*engagementServiceImpl, error) {
	if database == nil {
		return nil, gerror.New("marketing engagement service requires a database")
	}
	if random == nil {
		return nil, gerror.New("marketing engagement service requires a random source")
	}
	return &engagementServiceImpl{database: database, random: random}, nil
}

type marketingContactRow struct {
	CustomerNo      string     `orm:"customer_no"`
	ContactNo       string     `orm:"contact_no"`
	Status          string     `orm:"status_code"`
	Email           string     `orm:"email"`
	EmailVerifiedAt *time.Time `orm:"email_verified_at"`
	Phone           string     `orm:"phone"`
	PhoneVerifiedAt *time.Time `orm:"phone_verified_at"`
}

type marketingConsentEventRow struct {
	EventNo            string    `orm:"event_no"`
	RequestFingerprint string    `orm:"request_fingerprint"`
	OccurredAt         time.Time `orm:"occurred_at"`
}

func (s *engagementServiceImpl) SetConsent(
	ctx context.Context,
	input MarketingConsentInput,
) (*MarketingConsentResult, error) {
	normalized, idempotencyDigest, fingerprint, err := normalizeMarketingConsent(input)
	if err != nil {
		return nil, err
	}
	var result *MarketingConsentResult
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		contact, lockErr := lockMarketingContact(ctx, s.database, normalized.CustomerNo, normalized.ContactNo)
		if lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadMarketingConsentEvent(
			ctx, s.database, normalized.CustomerNo, normalized.ContactNo,
			normalized.Channel, idempotencyDigest,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeEngagementConflict)
			}
			projection, projectionErr := loadMarketingConsentProjection(
				ctx, s.database, normalized.CustomerNo, normalized.ContactNo, normalized.Channel,
			)
			if projectionErr != nil {
				return projectionErr
			}
			if projection == nil {
				return gerror.New("marketing consent event has no current projection")
			}
			result = projectMarketingConsent(
				existing, projection.Status, projection.Version, normalized, true,
			)
			return nil
		}
		if normalized.Action == MarketingConsentGranted &&
			(contact.Status != "active" || !marketingDestinationAvailable(contact, normalized.Channel)) {
			return bizerr.NewCode(CodeEngagementConflict)
		}
		eventNo, numberErr := newMarketingNumber(s.random, "mcev_")
		if numberErr != nil {
			return numberErr
		}
		databaseTime, timeErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return gerror.Wrap(timeErr, "read marketing consent database time")
		}
		occurredAt := databaseTime.Time()
		_, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_channel_consent_event (
    tenant_id, event_no, customer_no, contact_no, channel_code, action_code,
    policy_reference, policy_version, locale, source_code, evidence_digest,
    idempotency_key_digest, request_fingerprint, occurred_at
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			eventNo, normalized.CustomerNo, normalized.ContactNo, normalized.Channel,
			normalized.Action, normalized.PolicyReference, normalized.PolicyVersion,
			normalized.Locale, normalized.Source, normalized.EvidenceDigest,
			idempotencyDigest, fingerprint, occurredAt,
		)
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert marketing consent event")
		}
		status := "opted_out"
		enabled := false
		if normalized.Action == MarketingConsentGranted {
			status = "opted_in"
			enabled = true
		}
		versionValue, projectionErr := s.database.GetValue(ctx, `
INSERT INTO plugin_idc_core_marketing_channel_consent (
    tenant_id, customer_no, contact_no, channel_code, status_code,
    current_event_no, consent_occurred_at
) VALUES (0, ?, ?, ?, ?, ?, ?)
ON CONFLICT (customer_no, contact_no, channel_code) DO UPDATE SET
    status_code = EXCLUDED.status_code,
    current_event_no = EXCLUDED.current_event_no,
    consent_occurred_at = EXCLUDED.consent_occurred_at,
    version = plugin_idc_core_marketing_channel_consent.version + 1,
    updated_at = CURRENT_TIMESTAMP
RETURNING version`, normalized.CustomerNo, normalized.ContactNo, normalized.Channel,
			status, eventNo, occurredAt)
		if projectionErr != nil {
			return gerror.Wrap(projectionErr, "project marketing consent")
		}
		version := versionValue.Int64()
		_, preferenceErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_contact_preference (
    tenant_id, customer_no, contact_no, topic_code, channel_code, enabled
) VALUES (0, ?, ?, 'marketing', ?, ?)
ON CONFLICT (contact_no, topic_code, channel_code) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    version = plugin_idc_core_customer_contact_preference.version + 1,
    updated_at = CURRENT_TIMESTAMP`, normalized.CustomerNo, normalized.ContactNo,
			normalized.Channel, enabled)
		if preferenceErr != nil {
			return gerror.Wrap(preferenceErr, "project marketing contact preference")
		}
		result = &MarketingConsentResult{
			EventNo: eventNo, CustomerNo: normalized.CustomerNo, ContactNo: normalized.ContactNo,
			Channel: normalized.Channel, Status: status, Version: version,
			OccurredAt: occurredAt,
		}
		return nil
	})
	return engagementConsentResult(result, err)
}

type marketingConsentProjectionRow struct {
	Status  string `orm:"status_code"`
	Version int64  `orm:"version"`
}

func (s *engagementServiceImpl) SetQuietHours(
	ctx context.Context,
	input MarketingQuietHoursInput,
) (*MarketingQuietHours, error) {
	normalized, err := normalizeMarketingQuietHours(input)
	if err != nil {
		return nil, err
	}
	var result *MarketingQuietHours
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := lockMarketingCustomer(ctx, s.database, normalized.CustomerNo); lockErr != nil {
			return lockErr
		}
		var version int64
		if normalized.ExpectedVersion == 0 {
			versionValue, createErr := s.database.GetValue(ctx, `
INSERT INTO plugin_idc_core_marketing_quiet_hours (
    tenant_id, customer_no, timezone_name, start_minute, end_minute, enabled
) VALUES (0, ?, ?, ?, ?, ?) ON CONFLICT (customer_no) DO NOTHING
RETURNING version`, normalized.CustomerNo, normalized.TimezoneName,
				normalized.StartMinute, normalized.EndMinute, normalized.Enabled)
			if createErr != nil {
				return gerror.Wrap(createErr, "create marketing quiet hours")
			}
			version = versionValue.Int64()
		} else {
			versionValue, updateErr := s.database.GetValue(ctx, `
UPDATE plugin_idc_core_marketing_quiet_hours
SET timezone_name = ?, start_minute = ?, end_minute = ?, enabled = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND customer_no = ? AND version = ?
RETURNING version`, normalized.TimezoneName, normalized.StartMinute, normalized.EndMinute,
				normalized.Enabled, normalized.CustomerNo, normalized.ExpectedVersion)
			if updateErr != nil {
				return gerror.Wrap(updateErr, "update marketing quiet hours")
			}
			version = versionValue.Int64()
		}
		if version == 0 {
			return bizerr.NewCode(CodeEngagementConflict)
		}
		result = &MarketingQuietHours{
			CustomerNo: normalized.CustomerNo, TimezoneName: normalized.TimezoneName,
			StartMinute: normalized.StartMinute, EndMinute: normalized.EndMinute,
			Enabled: normalized.Enabled, Version: version,
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeEngagementUnavailable)
	}
	return result, nil
}

type marketingGateRow struct {
	Ordinal           int        `orm:"ordinal"`
	CampaignAvailable bool       `orm:"campaign_available"`
	SnapshotAvailable bool       `orm:"snapshot_available"`
	AudienceMember    bool       `orm:"audience_member"`
	CustomerActive    bool       `orm:"customer_active"`
	ContactActive     bool       `orm:"contact_active"`
	Email             string     `orm:"email"`
	EmailVerifiedAt   *time.Time `orm:"email_verified_at"`
	Phone             string     `orm:"phone"`
	PhoneVerifiedAt   *time.Time `orm:"phone_verified_at"`
	PreferenceEnabled *bool      `orm:"preference_enabled"`
	PreferenceVersion *int64     `orm:"preference_version"`
	ConsentStatus     *string    `orm:"consent_status"`
	ConsentEventNo    *string    `orm:"consent_event_no"`
	ConsentVersion    *int64     `orm:"consent_version"`
	QuietTimezone     *string    `orm:"quiet_timezone"`
	QuietStartMinute  *int       `orm:"quiet_start_minute"`
	QuietEndMinute    *int       `orm:"quiet_end_minute"`
	QuietEnabled      *bool      `orm:"quiet_enabled"`
	QuietVersion      *int64     `orm:"quiet_version"`
}

func (s *engagementServiceImpl) CheckSendBatch(
	ctx context.Context,
	inputs []MarketingSendGateInput,
) ([]MarketingSendDecision, error) {
	normalized, err := normalizeMarketingGateInputs(inputs)
	if err != nil {
		return nil, err
	}
	databaseTime, err := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "read marketing gate database time"), CodeEngagementUnavailable)
	}
	evaluatedAt := databaseTime.Time()
	values := make([]string, len(normalized))
	arguments := make([]any, 0, len(normalized)*6+1)
	for index, input := range normalized {
		values[index] = `(?, ?, ?, ?, ?, ?)`
		arguments = append(arguments, index, input.CampaignNo, input.SnapshotNo,
			input.CustomerNo, input.ContactNo, input.Channel)
	}
	query := `
WITH requested(ordinal, campaign_no, snapshot_no, customer_no, contact_no, channel_code) AS (
    VALUES ` + strings.Join(values, ",") + `
)
SELECT r.ordinal,
       (c.campaign_no IS NOT NULL) AS campaign_available,
       (s.snapshot_no IS NOT NULL) AS snapshot_available,
       (sm.customer_no IS NOT NULL) AS audience_member,
       (cs.customer_no IS NOT NULL) AS customer_active,
       (ct.contact_no IS NOT NULL) AS contact_active,
       coalesce(ct.email, '') AS email, ct.email_verified_at,
       coalesce(ct.phone, '') AS phone, ct.phone_verified_at,
       p.enabled AS preference_enabled, p.version AS preference_version,
       mc.status_code AS consent_status, mc.current_event_no AS consent_event_no,
       mc.version AS consent_version, q.timezone_name AS quiet_timezone,
       q.start_minute AS quiet_start_minute, q.end_minute AS quiet_end_minute,
       q.enabled AS quiet_enabled, q.version AS quiet_version
FROM requested r
LEFT JOIN plugin_idc_core_marketing_campaign c
  ON c.tenant_id = 0 AND c.campaign_no = r.campaign_no
 AND c.target_snapshot_no = r.snapshot_no AND c.status_code = 'active'
 AND c.starts_at <= ? AND c.ends_at > ?
LEFT JOIN plugin_idc_core_marketing_segment_snapshot s
  ON s.tenant_id = 0 AND s.snapshot_no = r.snapshot_no
 AND s.segment_no = c.target_segment_no AND s.status_code = 'complete'
LEFT JOIN plugin_idc_core_marketing_snapshot_member sm
  ON sm.tenant_id = 0 AND sm.snapshot_no = r.snapshot_no AND sm.customer_no = r.customer_no
LEFT JOIN plugin_idc_core_customer_subject cs
  ON cs.tenant_id = 0 AND cs.customer_no = r.customer_no AND cs.status_code = 'active'
LEFT JOIN plugin_idc_core_customer_contact ct
  ON ct.tenant_id = 0 AND ct.customer_no = r.customer_no
 AND ct.contact_no = r.contact_no AND ct.status_code = 'active'
LEFT JOIN plugin_idc_core_customer_contact_preference p
  ON p.tenant_id = 0 AND p.customer_no = r.customer_no AND p.contact_no = r.contact_no
 AND p.topic_code = 'marketing' AND p.channel_code = r.channel_code
LEFT JOIN plugin_idc_core_marketing_channel_consent mc
  ON mc.tenant_id = 0 AND mc.customer_no = r.customer_no
 AND mc.contact_no = r.contact_no AND mc.channel_code = r.channel_code
LEFT JOIN plugin_idc_core_marketing_quiet_hours q
  ON q.tenant_id = 0 AND q.customer_no = r.customer_no
ORDER BY r.ordinal ASC`
	// The campaign window uses the same authoritative database timestamp twice.
	arguments = append(arguments, evaluatedAt, evaluatedAt)
	var rows []marketingGateRow
	if err = s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "evaluate marketing send gates"), CodeEngagementUnavailable)
	}
	if len(rows) != len(normalized) {
		return nil, bizerr.WrapCode(gerror.New("marketing gate projection count mismatch"), CodeEngagementUnavailable)
	}
	decisions := make([]MarketingSendDecision, len(normalized))
	for index := range normalized {
		if rows[index].Ordinal != index {
			return nil, bizerr.WrapCode(
				gerror.New("marketing gate projection order mismatch"), CodeEngagementUnavailable,
			)
		}
		decision, decisionErr := buildMarketingDecision(normalized[index], rows[index], evaluatedAt)
		if decisionErr != nil {
			return nil, bizerr.WrapCode(decisionErr, CodeEngagementUnavailable)
		}
		decisions[index] = decision
	}
	return decisions, nil
}

func lockMarketingContact(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	contactNo string,
) (*marketingContactRow, error) {
	var contact *marketingContactRow
	if err := database.GetScan(ctx, &contact, `
SELECT customer_no, contact_no, status_code, email, email_verified_at, phone, phone_verified_at
FROM plugin_idc_core_customer_contact
WHERE tenant_id = 0 AND customer_no = ? AND contact_no = ? FOR UPDATE`,
		customerNo, contactNo); err != nil {
		return nil, gerror.Wrap(err, "lock marketing contact")
	}
	if contact == nil {
		return nil, bizerr.NewCode(CodeEngagementConflict)
	}
	return contact, nil
}

func lockMarketingCustomer(ctx context.Context, database gdb.DB, customerNo string) error {
	value, err := database.GetValue(ctx, `
SELECT customer_no FROM plugin_idc_core_customer_subject
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active' FOR UPDATE`, customerNo)
	if err != nil {
		return gerror.Wrap(err, "lock marketing customer")
	}
	if value.String() != customerNo {
		return bizerr.NewCode(CodeEngagementConflict)
	}
	return nil
}

func loadMarketingConsentEvent(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	contactNo string,
	channel string,
	idempotencyDigest string,
) (*marketingConsentEventRow, error) {
	var event *marketingConsentEventRow
	if err := database.GetScan(ctx, &event, `
SELECT event_no, request_fingerprint, occurred_at
FROM plugin_idc_core_marketing_channel_consent_event
WHERE tenant_id = 0 AND customer_no = ? AND contact_no = ?
  AND channel_code = ? AND idempotency_key_digest = ?`,
		customerNo, contactNo, channel, idempotencyDigest); err != nil {
		return nil, gerror.Wrap(err, "load marketing consent event")
	}
	return event, nil
}

func loadMarketingConsentProjection(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	contactNo string,
	channel string,
) (*marketingConsentProjectionRow, error) {
	var projection *marketingConsentProjectionRow
	if err := database.GetScan(ctx, &projection, `
SELECT status_code, version FROM plugin_idc_core_marketing_channel_consent
WHERE tenant_id = 0 AND customer_no = ? AND contact_no = ? AND channel_code = ?`,
		customerNo, contactNo, channel); err != nil {
		return nil, gerror.Wrap(err, "load marketing consent projection")
	}
	return projection, nil
}

func normalizeMarketingConsent(
	input MarketingConsentInput,
) (MarketingConsentInput, string, string, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.ContactNo = strings.TrimSpace(input.ContactNo)
	input.Channel = strings.TrimSpace(input.Channel)
	input.Action = strings.TrimSpace(input.Action)
	input.PolicyReference = strings.TrimSpace(input.PolicyReference)
	input.PolicyVersion = strings.TrimSpace(input.PolicyVersion)
	input.Locale = strings.TrimSpace(input.Locale)
	input.Source = strings.TrimSpace(input.Source)
	input.EvidenceDigest = strings.TrimSpace(input.EvidenceDigest)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !identifierPattern.MatchString(input.CustomerNo) || !identifierPattern.MatchString(input.ContactNo) ||
		!validMarketingChannel(input.Channel) ||
		(input.Action != MarketingConsentGranted && input.Action != MarketingConsentWithdrawn) ||
		!validBoundedText(input.PolicyReference, 256) || !validBoundedText(input.PolicyVersion, 64) ||
		(input.Locale != "zh-CN" && input.Locale != "en-US") || !validMarketingConsentSource(input.Source) ||
		!lowerHexDigest(input.EvidenceDigest) || !validBoundedText(input.IdempotencyKey, 128) {
		return MarketingConsentInput{}, "", "", bizerr.NewCode(CodeEngagementInvalid)
	}
	idempotencyDigest := sha256Hex("marketing-consent-idempotency-v1\x00" + input.IdempotencyKey)
	canonical, err := json.Marshal(struct {
		CustomerNo      string `json:"customer_no"`
		ContactNo       string `json:"contact_no"`
		Channel         string `json:"channel"`
		Action          string `json:"action"`
		PolicyReference string `json:"policy_reference"`
		PolicyVersion   string `json:"policy_version"`
		Locale          string `json:"locale"`
		Source          string `json:"source"`
		EvidenceDigest  string `json:"evidence_digest"`
	}{
		input.CustomerNo, input.ContactNo, input.Channel, input.Action,
		input.PolicyReference, input.PolicyVersion, input.Locale, input.Source,
		input.EvidenceDigest,
	})
	if err != nil {
		return MarketingConsentInput{}, "", "", gerror.Wrap(err, "encode marketing consent fingerprint")
	}
	return input, idempotencyDigest, sha256Hex(string(canonical)), nil
}

func normalizeMarketingQuietHours(input MarketingQuietHoursInput) (MarketingQuietHoursInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.TimezoneName = strings.TrimSpace(input.TimezoneName)
	if !identifierPattern.MatchString(input.CustomerNo) || input.ExpectedVersion < 0 ||
		input.StartMinute < 0 || input.StartMinute > 1439 ||
		input.EndMinute < 0 || input.EndMinute > 1439 ||
		(input.Enabled && input.StartMinute == input.EndMinute) {
		return MarketingQuietHoursInput{}, bizerr.NewCode(CodeEngagementInvalid)
	}
	if _, err := time.LoadLocation(input.TimezoneName); err != nil {
		return MarketingQuietHoursInput{}, bizerr.NewCode(CodeEngagementInvalid)
	}
	return input, nil
}

func normalizeMarketingGateInputs(inputs []MarketingSendGateInput) ([]MarketingSendGateInput, error) {
	if len(inputs) == 0 || len(inputs) > maximumMarketingGateBatch {
		return nil, bizerr.NewCode(CodeEngagementInvalid)
	}
	result := make([]MarketingSendGateInput, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for index, input := range inputs {
		input.CampaignNo = strings.TrimSpace(input.CampaignNo)
		input.SnapshotNo = strings.TrimSpace(input.SnapshotNo)
		input.CustomerNo = strings.TrimSpace(input.CustomerNo)
		input.ContactNo = strings.TrimSpace(input.ContactNo)
		input.Channel = strings.TrimSpace(input.Channel)
		input.ContentDigest = strings.TrimSpace(input.ContentDigest)
		if !identifierPattern.MatchString(input.CampaignNo) || !identifierPattern.MatchString(input.SnapshotNo) ||
			!identifierPattern.MatchString(input.CustomerNo) || !identifierPattern.MatchString(input.ContactNo) ||
			!validMarketingChannel(input.Channel) || !lowerHexDigest(input.ContentDigest) {
			return nil, bizerr.NewCode(CodeEngagementInvalid)
		}
		key := input.CampaignNo + "\x00" + input.SnapshotNo + "\x00" + input.CustomerNo +
			"\x00" + input.ContactNo + "\x00" + input.Channel
		if _, exists := seen[key]; exists {
			return nil, bizerr.NewCode(CodeEngagementInvalid)
		}
		seen[key] = struct{}{}
		result[index] = input
	}
	return result, nil
}

func buildMarketingDecision(
	input MarketingSendGateInput,
	row marketingGateRow,
	evaluatedAt time.Time,
) (MarketingSendDecision, error) {
	reason := MarketingGateAllowed
	contact := &marketingContactRow{
		Status: "active", Email: row.Email, EmailVerifiedAt: row.EmailVerifiedAt,
		Phone: row.Phone, PhoneVerifiedAt: row.PhoneVerifiedAt,
	}
	switch {
	case !row.CampaignAvailable:
		reason = MarketingGateCampaignUnavailable
	case !row.SnapshotAvailable:
		reason = MarketingGateSnapshotUnavailable
	case !row.AudienceMember:
		reason = MarketingGateOutsideAudience
	case !row.CustomerActive:
		reason = MarketingGateCustomerUnavailable
	case !row.ContactActive:
		reason = MarketingGateContactUnavailable
	case !marketingDestinationAvailable(contact, input.Channel):
		reason = MarketingGateDestinationUnavailable
	case row.ConsentStatus == nil:
		reason = MarketingGateConsentMissing
	case *row.ConsentStatus != "opted_in":
		reason = MarketingGateConsentWithdrawn
	case row.PreferenceEnabled == nil || !*row.PreferenceEnabled:
		reason = MarketingGatePreferenceDisabled
	default:
		inside, quietErr := insideMarketingQuietHours(row, evaluatedAt)
		if quietErr != nil {
			return MarketingSendDecision{}, quietErr
		}
		if inside {
			reason = MarketingGateQuietHours
		}
	}
	decision := MarketingSendDecision{
		CampaignNo: input.CampaignNo, SnapshotNo: input.SnapshotNo,
		CustomerNo: input.CustomerNo, ContactNo: input.ContactNo, Channel: input.Channel,
		Allowed: reason == MarketingGateAllowed, ReasonCode: reason, EvaluatedAt: evaluatedAt,
	}
	if row.ConsentEventNo != nil {
		decision.ConsentEventNo = *row.ConsentEventNo
	}
	if row.ConsentVersion != nil {
		decision.ConsentVersion = *row.ConsentVersion
	}
	if row.PreferenceVersion != nil {
		decision.PreferenceVersion = *row.PreferenceVersion
	}
	if row.QuietVersion != nil {
		decision.QuietHoursVersion = *row.QuietVersion
	}
	digestPayload := fmt.Sprintf(
		"marketing-send-decision-v1\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00%d\x00%s",
		input.CampaignNo, input.SnapshotNo, input.CustomerNo, input.ContactNo, input.Channel,
		input.ContentDigest, reason, decision.ConsentVersion, decision.PreferenceVersion,
		decision.QuietHoursVersion, evaluatedAt.UTC().UnixNano(), decision.ConsentEventNo,
	)
	decision.DecisionDigest = sha256Hex(digestPayload)
	return decision, nil
}

func insideMarketingQuietHours(row marketingGateRow, evaluatedAt time.Time) (bool, error) {
	if row.QuietEnabled == nil || !*row.QuietEnabled {
		return false, nil
	}
	if row.QuietTimezone == nil || row.QuietStartMinute == nil || row.QuietEndMinute == nil {
		return false, gerror.New("enabled marketing quiet hours are incomplete")
	}
	location, err := time.LoadLocation(*row.QuietTimezone)
	if err != nil {
		return false, gerror.Wrap(err, "load persisted marketing quiet-hours timezone")
	}
	local := evaluatedAt.In(location)
	minute := local.Hour()*60 + local.Minute()
	start := *row.QuietStartMinute
	end := *row.QuietEndMinute
	if start < end {
		return minute >= start && minute < end, nil
	}
	return minute >= start || minute < end, nil
}

func marketingDestinationAvailable(contact *marketingContactRow, channel string) bool {
	switch channel {
	case "email":
		return contact.Email != "" && contact.EmailVerifiedAt != nil
	case "sms":
		return contact.Phone != "" && contact.PhoneVerifiedAt != nil
	case "in_app", "wechat_official", "wechat_work":
		return true
	default:
		return false
	}
}

func projectMarketingConsent(
	event *marketingConsentEventRow,
	status string,
	version int64,
	input MarketingConsentInput,
	reused bool,
) *MarketingConsentResult {
	return &MarketingConsentResult{
		EventNo: event.EventNo, CustomerNo: input.CustomerNo, ContactNo: input.ContactNo,
		Channel: input.Channel, Status: status, Version: version,
		OccurredAt: event.OccurredAt, Reused: reused,
	}
}

func engagementConsentResult(
	result *MarketingConsentResult,
	err error,
) (*MarketingConsentResult, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeEngagementUnavailable)
}

func validMarketingChannel(value string) bool {
	return value == "in_app" || value == "email" || value == "sms" ||
		value == "wechat_official" || value == "wechat_work"
}

func validMarketingConsentSource(value string) bool {
	return value == "portal" || value == "admin" || value == "migration" || value == "notify"
}

func validBoundedText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func lowerHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
