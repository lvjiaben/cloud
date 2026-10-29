// This file owns signed webhook events and bounded, atomically leased delivery.
// Claiming uses set operations while retaining per-event unpredictable leases.

package channel

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
)

// SignWebhook returns the stable signature format used for one versioned
// channel event. The version and event id are part of the signed bytes, so a
// payload cannot be replayed under another event or protocol version.
func SignWebhook(secret []byte, version, eventID string, payload []byte) (string, error) {
	if len(secret) == 0 || version == "" || eventID == "" {
		return "", errors.New("webhook signing inputs are required")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(version))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(eventID))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifyWebhook accepts only the exact signature produced by SignWebhook and
// compares it in constant time to avoid timing leaks.
func VerifyWebhook(secret []byte, version, eventID string, payload []byte, signature string) bool {
	expected, err := SignWebhook(secret, version, eventID, payload)
	if err != nil || signature == "" {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(signature))
}

// WebhookEventInput is the immutable business fact used to create one event.
// The signing secret is deliberately not part of the persisted model.
type WebhookEventInput struct {
	ChannelID  string
	Version    string
	EventType  string
	SubjectRef string
	Sequence   int64
	Payload    []byte
	Secret     []byte
}

// WebhookDelivery is the safe projection returned to a delivery worker or a
// downstream replay request. It never contains a signing secret.
type WebhookDelivery struct {
	ID            int64
	ChannelID     string
	EventID       string
	Version       string
	EventType     string
	SubjectRef    string
	Sequence      int64
	Payload       json.RawMessage
	PayloadHash   string
	Signature     string
	Status        string
	Attempts      int
	NextAttemptAt *time.Time
	LeaseToken    string
	LeaseExpires  *time.Time
	DeliveredAt   *time.Time
	LastError     string
	CreatedAt     time.Time
}

const (
	WebhookStatusPending    = "pending"
	WebhookStatusDelivering = "delivering"
	WebhookStatusRetryWait  = "retry_wait"
	WebhookStatusDelivered  = "delivered"
	WebhookStatusFailed     = "failed"
	// One initial delivery plus six retry windows (1, 2, 4, 8, 16 and 30
	// minutes) is the bounded policy for a transient downstream failure.
	WebhookMaxAttempts = 7
)

// StableWebhookEventID derives the same event number after a restart or a
// replay. All business identity inputs are included in the digest.
func StableWebhookEventID(in WebhookEventInput) (string, error) {
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.Version = strings.TrimSpace(in.Version)
	in.EventType = strings.TrimSpace(in.EventType)
	in.SubjectRef = strings.TrimSpace(in.SubjectRef)
	if in.ChannelID == "" || in.Version == "" || in.EventType == "" || in.SubjectRef == "" || in.Sequence < 0 {
		return "", errors.New("webhook event identity is required")
	}
	if len(in.ChannelID) > 80 || len(in.Version) > 20 || len(in.EventType) > 80 || len(in.SubjectRef) > 160 {
		return "", errors.New("webhook event identity is too long")
	}
	b, err := json.Marshal(struct {
		ChannelID  string `json:"channel_id"`
		Version    string `json:"version"`
		EventType  string `json:"event_type"`
		SubjectRef string `json:"subject_ref"`
		Sequence   int64  `json:"sequence"`
	}{in.ChannelID, in.Version, in.EventType, in.SubjectRef, in.Sequence})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "evt_" + hex.EncodeToString(sum[:]), nil
}

// WebhookRetryAt returns a bounded exponential backoff. It is deterministic
// so workers on different nodes never disagree about the next due time.
func WebhookRetryAt(now time.Time, attempts int) (time.Time, bool) {
	if attempts < 1 || attempts >= WebhookMaxAttempts {
		return time.Time{}, false
	}
	delay := time.Minute << (attempts - 1)
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	return now.Add(delay), true
}

func newWebhookLeaseToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// EnqueueWebhook atomically records an immutable event. A duplicate event
// number is safe only when its version, payload and signature are identical.
func (s *service) EnqueueWebhook(ctx context.Context, in WebhookEventInput) (WebhookDelivery, error) {
	if s == nil || s.db == nil {
		return WebhookDelivery{}, errors.New("webhook database is unavailable")
	}
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.Version = strings.TrimSpace(in.Version)
	in.EventType = strings.TrimSpace(in.EventType)
	in.SubjectRef = strings.TrimSpace(in.SubjectRef)
	if len(in.Payload) == 0 || len(in.Payload) > 1<<20 || !json.Valid(in.Payload) {
		return WebhookDelivery{}, errors.New("webhook payload must be valid json")
	}
	eventID, err := StableWebhookEventID(in)
	if err != nil {
		return WebhookDelivery{}, err
	}
	signature, err := SignWebhook(in.Secret, in.Version, eventID, in.Payload)
	if err != nil {
		return WebhookDelivery{}, err
	}
	hash := sha256.Sum256(in.Payload)
	payloadHash := hex.EncodeToString(hash[:])
	row := g.Map{
		"channel_id": in.ChannelID, "event_id": eventID, "event_version": in.Version,
		"event_type": in.EventType, "subject_ref": in.SubjectRef, "sequence_no": in.Sequence,
		"payload": string(in.Payload), "payload_text": string(in.Payload), "payload_hash": payloadHash, "signature": signature,
		"status": WebhookStatusPending, "attempts": 0, "next_attempt_at": gdb.Raw("CURRENT_TIMESTAMP"),
	}
	if _, err = s.db.Model("plugin_idc_channel_webhook").Ctx(ctx).Data(row).Insert(); err != nil {
		existing, readErr := s.db.Model("plugin_idc_channel_webhook").Ctx(ctx).Where("channel_id=? AND event_id=?", in.ChannelID, eventID).One()
		if readErr != nil || existing == nil {
			return WebhookDelivery{}, err
		}
		if existing["event_version"].String() != in.Version || existing["payload_hash"].String() != payloadHash || existing["signature"].String() != signature {
			return WebhookDelivery{}, errors.New("webhook event id already exists with different payload")
		}
		return webhookDeliveryFromRecord(existing), nil
	}
	existing, err := s.db.Model("plugin_idc_channel_webhook").Ctx(ctx).Where("channel_id=? AND event_id=?", in.ChannelID, eventID).One()
	if err != nil || existing == nil {
		if err == nil {
			err = errors.New("webhook event was not persisted")
		}
		return WebhookDelivery{}, err
	}
	return webhookDeliveryFromRecord(existing), nil
}

// ClaimWebhookDeliveries leases a bounded batch using PostgreSQL's authority
// clock and SKIP LOCKED, preventing two workers from delivering one row.
func (s *service) ClaimWebhookDeliveries(ctx context.Context, limit int, lease time.Duration) ([]WebhookDelivery, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("webhook database is unavailable")
	}
	if limit < 1 {
		return []WebhookDelivery{}, nil
	}
	if limit > 32 {
		limit = 32
	}
	if lease <= 0 || lease > time.Hour {
		return nil, errors.New("webhook lease duration is invalid")
	}
	var out []WebhookDelivery
	err := s.db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		// A worker that disappeared must not strand the event forever. Requeue
		// only an expired lease; the event id makes any late delivery harmless
		// for a downstream consumer that deduplicates correctly.
		if _, err := s.db.Model("plugin_idc_channel_webhook").Ctx(txCtx).
			Data("status=?,next_attempt_at=CURRENT_TIMESTAMP,lease_token=?,lease_expires_at=NULL", WebhookStatusRetryWait, "").
			Where("status=? AND lease_expires_at IS NOT NULL AND lease_expires_at<=CURRENT_TIMESTAMP AND attempts<?", WebhookStatusDelivering, WebhookMaxAttempts).Update(); err != nil {
			return err
		}
		if _, err := s.db.Model("plugin_idc_channel_webhook").Ctx(txCtx).
			Data("status=?,lease_token=?,lease_expires_at=NULL,last_error=?", WebhookStatusFailed, "", "delivery lease expired after retry budget").
			Where("status=? AND lease_expires_at IS NOT NULL AND lease_expires_at<=CURRENT_TIMESTAMP AND attempts>=?", WebhookStatusDelivering, WebhookMaxAttempts).Update(); err != nil {
			return err
		}
		var rows []gdb.Record
		if err := s.db.Model("plugin_idc_channel_webhook").Ctx(txCtx).
			Where("status IN (?,?) AND (next_attempt_at IS NULL OR next_attempt_at<=CURRENT_TIMESTAMP) AND (lease_expires_at IS NULL OR lease_expires_at<=CURRENT_TIMESTAMP)", WebhookStatusPending, WebhookStatusRetryWait).
			OrderAsc("next_attempt_at").OrderAsc("id").Limit(limit).LockUpdateSkipLocked().Scan(&rows); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		var (
			ids       = make([]int64, 0, len(rows))
			branches  = make([]string, 0, len(rows))
			arguments = []any{WebhookStatusDelivering}
		)
		for _, row := range rows {
			token, tokenErr := newWebhookLeaseToken()
			if tokenErr != nil {
				return tokenErr
			}
			ids = append(ids, row["id"].Int64())
			branches = append(branches, "WHEN ? THEN ?")
			arguments = append(arguments, row["id"].Int64(), token)
		}
		// CASE is portable and lets every event retain its own random token.
		// The same locked row set is updated and read once, independent of N.
		assignment := "status=?,attempts=attempts+1,lease_token=CASE id " + strings.Join(branches, " ") + " ELSE lease_token END,lease_expires_at=" + fmt.Sprintf("CURRENT_TIMESTAMP + INTERVAL '%d milliseconds'", lease.Milliseconds())
		data := append([]any{assignment}, arguments...)
		updated, updateErr := s.db.Model("plugin_idc_channel_webhook").Ctx(txCtx).Data(data...).WhereIn("id", ids).Where("status IN (?,?)", WebhookStatusPending, WebhookStatusRetryWait).Update()
		if updateErr != nil {
			return updateErr
		}
		affected, countErr := updated.RowsAffected()
		if countErr != nil {
			return countErr
		}
		if affected != int64(len(rows)) {
			return gerror.New("webhook batch lease changed")
		}
		var claimed []gdb.Record
		if readErr := s.db.Model("plugin_idc_channel_webhook").Ctx(txCtx).WhereIn("id", ids).Where("status", WebhookStatusDelivering).OrderAsc("next_attempt_at").OrderAsc("id").Scan(&claimed); readErr != nil {
			return readErr
		}
		if len(claimed) != len(rows) {
			return gerror.New("webhook batch lease projection changed")
		}
		for _, row := range claimed {
			out = append(out, webhookDeliveryFromRecord(row))
		}
		return nil
	})
	return out, err
}

// MarkWebhookDelivered closes a leased delivery only when its lease token is
// still current; late workers therefore cannot overwrite a newer attempt.
func (s *service) MarkWebhookDelivered(ctx context.Context, channelID, eventID, leaseToken string) error {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(eventID) == "" || strings.TrimSpace(leaseToken) == "" {
		return errors.New("webhook delivery lease is required")
	}
	result, err := s.db.Model("plugin_idc_channel_webhook").Ctx(ctx).Data(g.Map{"status": WebhookStatusDelivered, "delivered_at": gdb.Raw("CURRENT_TIMESTAMP"), "lease_token": "", "lease_expires_at": nil, "next_attempt_at": nil, "last_error": "", "updated_at": gdb.Raw("CURRENT_TIMESTAMP")}).Where("channel_id=? AND event_id=? AND status=? AND lease_token=?", channelID, eventID, WebhookStatusDelivering, leaseToken).Update()
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return errors.New("webhook delivery lease is stale")
	}
	return nil
}

// ScheduleWebhookRetry moves a leased event to a bounded retry window or to
// the terminal failed state after the retry budget is exhausted.
func (s *service) ScheduleWebhookRetry(ctx context.Context, channelID, eventID, leaseToken, failure string) error {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(eventID) == "" || strings.TrimSpace(leaseToken) == "" {
		return errors.New("webhook delivery lease is required")
	}
	row, err := s.db.Model("plugin_idc_channel_webhook").Ctx(ctx).Where("channel_id=? AND event_id=? AND status=? AND lease_token=?", channelID, eventID, WebhookStatusDelivering, leaseToken).One()
	if err != nil {
		return err
	}
	if row == nil {
		return errors.New("webhook delivery lease is stale")
	}
	next, ok := WebhookRetryAt(time.Time{}, row["attempts"].Int())
	status := WebhookStatusRetryWait
	if !ok {
		status = WebhookStatusFailed
		next = time.Time{}
	}
	data := g.Map{"status": status, "next_attempt_at": nil, "lease_token": "", "lease_expires_at": nil, "last_error": normalizeWebhookFailure(failure), "updated_at": gdb.Raw("CURRENT_TIMESTAMP")}
	if ok {
		data["next_attempt_at"] = gdb.Raw(fmt.Sprintf("CURRENT_TIMESTAMP + INTERVAL '%d seconds'", int(next.Sub(time.Time{}).Seconds())))
	}
	_, err = s.db.Model("plugin_idc_channel_webhook").Ctx(ctx).Data(data).Where("channel_id=? AND event_id=? AND status=? AND lease_token=?", channelID, eventID, WebhookStatusDelivering, leaseToken).Update()
	return err
}

// normalizeWebhookFailure stores only a closed failure code, never provider
// response text that might contain credentials or customer data.
func normalizeWebhookFailure(failure string) string {
	switch strings.ToLower(strings.TrimSpace(failure)) {
	case "timeout", "transport_unavailable", "remote_5xx", "remote_429", "malformed_response":
		return strings.ToLower(strings.TrimSpace(failure))
	default:
		return "delivery_failed"
	}
}

// GetWebhookEvent returns one channel-scoped immutable event for downstream
// replay. Unknown channel/event pairs intentionally share the same error.
func (s *service) GetWebhookEvent(ctx context.Context, channelID, eventID string) (WebhookDelivery, error) {
	channelID, eventID = strings.TrimSpace(channelID), strings.TrimSpace(eventID)
	if channelID == "" || eventID == "" {
		return WebhookDelivery{}, bizerr.NewCode(codeWebhookNotFound)
	}
	row, err := s.db.Model("plugin_idc_channel_webhook").Ctx(ctx).Where("channel_id=? AND event_id=?", channelID, eventID).One()
	if err != nil {
		return WebhookDelivery{}, err
	}
	if row == nil {
		return WebhookDelivery{}, bizerr.NewCode(codeWebhookNotFound)
	}
	return webhookDeliveryFromRecord(row), nil
}

func webhookDeliveryFromRecord(row gdb.Record) WebhookDelivery {
	var payload json.RawMessage
	value := row["payload_text"].Bytes()
	if len(value) == 0 {
		value = row["payload"].Bytes()
	}
	if len(value) > 0 {
		payload = append(json.RawMessage(nil), value...)
	}
	return WebhookDelivery{ID: row["id"].Int64(), ChannelID: row["channel_id"].String(), EventID: row["event_id"].String(), Version: row["event_version"].String(), EventType: row["event_type"].String(), SubjectRef: row["subject_ref"].String(), Sequence: row["sequence_no"].Int64(), Payload: payload, PayloadHash: row["payload_hash"].String(), Signature: row["signature"].String(), Status: row["status"].String(), Attempts: row["attempts"].Int(), NextAttemptAt: optionalWebhookTime(row["next_attempt_at"]), LeaseToken: row["lease_token"].String(), LeaseExpires: optionalWebhookTime(row["lease_expires_at"]), DeliveredAt: optionalWebhookTime(row["delivered_at"]), LastError: row["last_error"].String(), CreatedAt: row["created_at"].Time()}
}

func optionalWebhookTime(value *gvar.Var) *time.Time {
	if value == nil || value.IsNil() {
		return nil
	}
	t := value.Time()
	if t.IsZero() {
		return nil
	}
	return &t
}
