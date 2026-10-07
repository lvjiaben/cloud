package customercontact

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

var (
	CodeInvalid             = bizerr.MustDefine("IDC_CUSTOMER_CONTACT_INVALID", "The customer contact request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible       = bizerr.MustDefine("IDC_CUSTOMER_CONTACT_NOT_ACCESSIBLE", "The customer contact is not accessible", gcode.CodeNotAuthorized)
	CodeVersionConflict     = bizerr.MustDefine("IDC_CUSTOMER_CONTACT_VERSION_CONFLICT", "The customer contact has changed", gcode.CodeInvalidOperation)
	CodeIdempotencyConflict = bizerr.MustDefine("IDC_CUSTOMER_CONTACT_IDEMPOTENCY_CONFLICT", "The customer contact request conflicts with an earlier request", gcode.CodeInvalidOperation)
	CodeUnavailable         = bizerr.MustDefine("IDC_CUSTOMER_CONTACT_UNAVAILABLE", "The customer contact service is temporarily unavailable", gcode.CodeInternalError)
)

const (
	scopeSave    = "customer.contact.save"
	scopeArchive = "customer.contact.archive"
)

var (
	countryCallingCodePattern = regexp.MustCompile(`^\+[1-9][0-9]{0,3}$`)
	phonePattern              = regexp.MustCompile(`^[0-9]{4,20}$`)
	contactNumberPattern      = regexp.MustCompile(`^CTC_[0-9a-f]{32}$`)
)

type serviceImpl struct {
	database  gdb.DB
	access    *customer.AccessGuard
	audit     audit.Writer
	lookupKey []byte
}

func New(database gdb.DB, access *customer.AccessGuard, auditWriter audit.Writer, lookupKey []byte) (Service, error) {
	if database == nil || access == nil || auditWriter == nil || len(lookupKey) < 32 {
		return nil, gerror.New("customer contact service requires all dependencies")
	}
	return &serviceImpl{database: database, access: access, audit: auditWriter, lookupKey: append([]byte(nil), lookupKey...)}, nil
}

type contactRow struct {
	ContactNo        string     `orm:"contact_no"`
	DisplayName      string     `orm:"display_name"`
	Email            string     `orm:"email"`
	EmailVerifiedAt  *time.Time `orm:"email_verified_at"`
	PhoneCountryCode string     `orm:"phone_country_code"`
	Phone            string     `orm:"phone"`
	PhoneVerifiedAt  *time.Time `orm:"phone_verified_at"`
	PreferredLocale  string     `orm:"preferred_locale"`
	StatusCode       string     `orm:"status_code"`
	Version          int64      `orm:"version"`
	CreatedAt        time.Time  `orm:"created_at"`
	UpdatedAt        time.Time  `orm:"updated_at"`
}

func (s *serviceImpl) List(ctx context.Context) ([]Contact, error) {
	audience, err := s.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []contactRow
	if err = s.database.GetScan(ctx, &rows, `SELECT contact_no,display_name,email,email_verified_at,phone_country_code,phone,phone_verified_at,preferred_locale,status_code,version,created_at,updated_at FROM plugin_idc_core_customer_contact WHERE tenant_id=0 AND customer_no=? ORDER BY CASE status_code WHEN 'active' THEN 0 ELSE 1 END,id LIMIT 100`, audience.CustomerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	items := make([]Contact, 0, len(rows))
	for _, row := range rows {
		item, projectErr := project(row)
		if projectErr != nil {
			return nil, bizerr.WrapCode(projectErr, CodeUnavailable)
		}
		items = append(items, item)
	}
	return items, nil
}

type normalizedSave struct {
	contactNo, displayName, email, phoneCountryCode, phone, locale, idempotencyKey string
	expectedVersion                                                                int64
}

func (s *serviceImpl) Save(ctx context.Context, input SaveInput) (*Contact, error) {
	audience, err := s.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeSave(input)
	if err != nil {
		return nil, err
	}
	fingerprint := digest("customer-contact-save-fingerprint-v1", strings.Join([]string{
		normalized.contactNo, normalized.displayName, normalized.email,
		normalized.phoneCountryCode, normalized.phone, normalized.locale,
		strconv.FormatInt(normalized.expectedVersion, 10),
	}, "\x00"))
	var result *Contact
	err = s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		reservation, reserveErr := reserve(tx, scopeSave, audience.UserID, audience.CustomerNo, normalized.idempotencyKey, fingerprint)
		if reserveErr != nil {
			return reserveErr
		}
		if reservation.replay {
			result, reserveErr = loadReplay(tx, reservation, audience.CustomerNo)
			return reserveErr
		}
		if normalized.contactNo == "" {
			result, reserveErr = s.create(tx, audience.UserID, audience.CustomerNo, normalized)
		} else {
			result, reserveErr = s.update(tx, audience.UserID, audience.CustomerNo, normalized)
		}
		if reserveErr != nil {
			return reserveErr
		}
		if _, reserveErr = s.audit.AppendTransition(txCtx, audit.TransitionInput{
			Fields: audit.Fields{IdempotencyScope: scopeSave, IdempotencyKey: normalized.idempotencyKey,
				OperationCode: scopeSave, TargetType: "customer_contact", TargetNo: result.ContactNo,
				CustomerNo: audience.CustomerNo, Result: audit.ResultSucceeded, Risk: audit.RiskLow},
			BeforeSnapshot: map[string]any{"version": normalized.expectedVersion},
			AfterSnapshot:  safeSnapshot(result),
		}); reserveErr != nil {
			return reserveErr
		}
		return complete(tx, reservation.id, result.ContactNo, result.Version)
	})
	return mapResult(result, err)
}

func (s *serviceImpl) Archive(ctx context.Context, input ArchiveInput) (*Contact, error) {
	audience, err := s.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	contactNo := strings.TrimSpace(input.ContactNo)
	key := strings.TrimSpace(input.IdempotencyKey)
	if !contactNumberPattern.MatchString(contactNo) || input.ExpectedVersion < 1 || len(key) < 16 || len(key) > 128 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	fingerprint := digest("customer-contact-archive-fingerprint-v1", contactNo+"\x00"+strconv.FormatInt(input.ExpectedVersion, 10))
	var result *Contact
	err = s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		reservation, reserveErr := reserve(tx, scopeArchive, audience.UserID, audience.CustomerNo, key, fingerprint)
		if reserveErr != nil {
			return reserveErr
		}
		if reservation.replay {
			result, reserveErr = loadReplay(tx, reservation, audience.CustomerNo)
			return reserveErr
		}
		before, loadErr := load(tx, audience.CustomerNo, contactNo, true)
		if loadErr != nil {
			return bizerr.WrapCode(loadErr, CodeNotAccessible)
		}
		if before == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if before.StatusCode != string(StatusActive) || before.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeVersionConflict)
		}
		now, timeErr := databaseTime(tx)
		if timeErr != nil {
			return timeErr
		}
		updated, updateErr := tx.Exec(`UPDATE plugin_idc_core_customer_contact SET status_code='archived',version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND contact_no=? AND status_code='active' AND version=?`, audience.UserID, now, audience.CustomerNo, contactNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := updated.RowsAffected(); affectedErr != nil || affected != 1 {
			return bizerr.WrapCode(affectedErr, CodeVersionConflict)
		}
		if _, updateErr = tx.Exec(`UPDATE plugin_idc_core_customer_contact_role SET status_code='revoked',revoked_by=?,revoked_at=COALESCE(revoked_at,?),updated_at=? WHERE tenant_id=0 AND customer_no=? AND contact_no=? AND status_code='active'`, audience.UserID, now, now, audience.CustomerNo, contactNo); updateErr != nil {
			return updateErr
		}
		after, loadErr := load(tx, audience.CustomerNo, contactNo, false)
		if loadErr != nil || after == nil {
			return gerror.Wrap(loadErr, "load archived customer contact")
		}
		projected, projectErr := project(*after)
		if projectErr != nil {
			return projectErr
		}
		result = &projected
		if _, updateErr = s.audit.AppendTransition(txCtx, audit.TransitionInput{
			Fields: audit.Fields{IdempotencyScope: scopeArchive, IdempotencyKey: key,
				OperationCode: scopeArchive, TargetType: "customer_contact", TargetNo: contactNo,
				CustomerNo: audience.CustomerNo, Result: audit.ResultSucceeded, Risk: audit.RiskLow},
			BeforeSnapshot: safeRowSnapshot(before), AfterSnapshot: safeSnapshot(result),
		}); updateErr != nil {
			return updateErr
		}
		return complete(tx, reservation.id, result.ContactNo, result.Version)
	})
	return mapResult(result, err)
}

func (s *serviceImpl) create(tx gdb.TX, userID int, customerNo string, input normalizedSave) (*Contact, error) {
	contactNo, err := newContactNo()
	if err != nil {
		return nil, err
	}
	now, err := databaseTime(tx)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`INSERT INTO plugin_idc_core_customer_contact (tenant_id,contact_no,customer_no,display_name,email,email_lookup_digest,phone_country_code,phone,phone_lookup_digest,preferred_locale,status_code,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,'active',1,?,?,?,?)`,
		contactNo, customerNo, input.displayName, input.email, s.destinationDigest("email", input.email),
		input.phoneCountryCode, input.phone, s.destinationDigest("phone", input.phoneCountryCode+input.phone),
		input.locale, userID, userID, now, now)
	if err != nil {
		return nil, err
	}
	row, err := load(tx, customerNo, contactNo, false)
	if err != nil || row == nil {
		return nil, gerror.Wrap(err, "load created customer contact")
	}
	item, err := project(*row)
	return &item, err
}

func (s *serviceImpl) update(tx gdb.TX, userID int, customerNo string, input normalizedSave) (*Contact, error) {
	before, err := load(tx, customerNo, input.contactNo, true)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if before == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if before.StatusCode != string(StatusActive) || before.Version != input.expectedVersion {
		return nil, bizerr.NewCode(CodeVersionConflict)
	}
	now, err := databaseTime(tx)
	if err != nil {
		return nil, err
	}
	emailChanged := !strings.EqualFold(before.Email, input.email)
	phoneChanged := before.PhoneCountryCode != input.phoneCountryCode || before.Phone != input.phone
	result, err := tx.Exec(`UPDATE plugin_idc_core_customer_contact SET display_name=?,email=?,email_lookup_digest=?,email_verified_at=CASE WHEN ? THEN NULL ELSE email_verified_at END,phone_country_code=?,phone=?,phone_lookup_digest=?,phone_verified_at=CASE WHEN ? THEN NULL ELSE phone_verified_at END,preferred_locale=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND contact_no=? AND status_code='active' AND version=?`,
		input.displayName, input.email, s.destinationDigest("email", input.email), emailChanged,
		input.phoneCountryCode, input.phone, s.destinationDigest("phone", input.phoneCountryCode+input.phone), phoneChanged,
		input.locale, userID, now, customerNo, input.contactNo, input.expectedVersion)
	if err != nil {
		return nil, err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return nil, bizerr.WrapCode(affectedErr, CodeVersionConflict)
	}
	after, err := load(tx, customerNo, input.contactNo, false)
	if err != nil || after == nil {
		return nil, gerror.Wrap(err, "load updated customer contact")
	}
	item, err := project(*after)
	return &item, err
}

type reservation struct {
	id       int64
	replay   bool
	resource string
	version  int64
}

func reserve(tx gdb.TX, scope string, userID int, customerNo, key, fingerprint string) (reservation, error) {
	caller := digest("customer-contact-caller-v1", customerNo+"\x00"+strconv.Itoa(userID))
	keyDigest := digest("customer-contact-key-v1", key)
	now, err := databaseTime(tx)
	if err != nil {
		return reservation{}, err
	}
	result, err := tx.Exec(`INSERT INTO plugin_idc_core_idempotency_record (tenant_id,scope,caller_scope_digest,idempotency_key_digest,request_fingerprint,status_code,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,'in_progress',?,?,?) ON CONFLICT (tenant_id,scope,caller_scope_digest,idempotency_key_digest) DO NOTHING`, scope, caller, keyDigest, fingerprint, now.Add(30*24*time.Hour), now, now)
	if err != nil {
		return reservation{}, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return reservation{}, err
	}
	type row struct {
		ID                 int64  `orm:"id"`
		RequestFingerprint string `orm:"request_fingerprint"`
		StatusCode         string `orm:"status_code"`
		ResourceNo         string `orm:"resource_no"`
		ResponseSnapshot   string `orm:"response_snapshot"`
	}
	var stored *row
	if err = tx.GetScan(&stored, `SELECT id,request_fingerprint,status_code,resource_no,response_snapshot::text FROM plugin_idc_core_idempotency_record WHERE tenant_id=0 AND scope=? AND caller_scope_digest=? AND idempotency_key_digest=? FOR UPDATE`, scope, caller, keyDigest); err != nil || stored == nil {
		return reservation{}, gerror.Wrap(err, "load customer contact idempotency reservation")
	}
	if stored.RequestFingerprint != fingerprint {
		return reservation{}, bizerr.NewCode(CodeIdempotencyConflict)
	}
	if inserted == 1 {
		return reservation{id: stored.ID}, nil
	}
	if stored.StatusCode != "succeeded" || !contactNumberPattern.MatchString(stored.ResourceNo) {
		return reservation{}, bizerr.NewCode(CodeUnavailable)
	}
	var snapshot struct {
		Version int64 `json:"version"`
	}
	if err = json.Unmarshal([]byte(stored.ResponseSnapshot), &snapshot); err != nil || snapshot.Version < 1 {
		return reservation{}, bizerr.NewCode(CodeUnavailable)
	}
	return reservation{id: stored.ID, replay: true, resource: stored.ResourceNo, version: snapshot.Version}, nil
}

func complete(tx gdb.TX, id int64, contactNo string, version int64) error {
	snapshot, err := json.Marshal(map[string]any{"version": version})
	if err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE plugin_idc_core_idempotency_record SET status_code='succeeded',resource_type='customer_contact',resource_no=?,response_status_code=200,response_snapshot=?::jsonb,completed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND id=? AND status_code='in_progress'`, contactNo, string(snapshot), id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.Wrap(err, "complete customer contact idempotency reservation")
	}
	return nil
}

func databaseTime(tx gdb.TX) (time.Time, error) {
	value, err := tx.GetValue(`SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, err
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("database returned an empty current timestamp")
	}
	return now, nil
}

func loadReplay(tx gdb.TX, value reservation, customerNo string) (*Contact, error) {
	row, err := load(tx, customerNo, value.resource, false)
	if err != nil || row == nil || row.Version < value.version {
		return nil, gerror.Wrap(err, "load customer contact replay")
	}
	item, err := project(*row)
	return &item, err
}

func load(tx gdb.TX, customerNo, contactNo string, lock bool) (*contactRow, error) {
	query := `SELECT contact_no,display_name,email,email_verified_at,phone_country_code,phone,phone_verified_at,preferred_locale,status_code,version,created_at,updated_at FROM plugin_idc_core_customer_contact WHERE tenant_id=0 AND customer_no=? AND contact_no=?`
	if lock {
		query += ` FOR UPDATE`
	}
	var row *contactRow
	err := tx.GetScan(&row, query, customerNo, contactNo)
	return row, err
}

func normalizeSave(input SaveInput) (normalizedSave, error) {
	value := normalizedSave{
		contactNo: strings.TrimSpace(input.ContactNo), displayName: strings.TrimSpace(input.DisplayName),
		email: strings.ToLower(strings.TrimSpace(input.Email)), phoneCountryCode: strings.TrimSpace(input.PhoneCountryCode),
		phone: strings.ReplaceAll(strings.TrimSpace(input.Phone), " ", ""), locale: strings.TrimSpace(input.PreferredLocale),
		expectedVersion: input.ExpectedVersion, idempotencyKey: strings.TrimSpace(input.IdempotencyKey),
	}
	if value.contactNo != "" && !contactNumberPattern.MatchString(value.contactNo) {
		return normalizedSave{}, bizerr.NewCode(CodeInvalid)
	}
	if (value.contactNo == "" && value.expectedVersion != 0) || (value.contactNo != "" && value.expectedVersion < 1) ||
		len(value.displayName) < 1 || len(value.displayName) > 256 ||
		(value.locale != "zh-CN" && value.locale != "en-US") ||
		len(value.idempotencyKey) < 16 || len(value.idempotencyKey) > 128 {
		return normalizedSave{}, bizerr.NewCode(CodeInvalid)
	}
	if value.email == "" && value.phone == "" {
		return normalizedSave{}, bizerr.NewCode(CodeInvalid)
	}
	if value.email != "" {
		parsed, err := mail.ParseAddress(value.email)
		if err != nil || parsed.Address != value.email || len(value.email) > 320 {
			return normalizedSave{}, bizerr.NewCode(CodeInvalid)
		}
	}
	if value.phone == "" {
		if value.phoneCountryCode != "" {
			return normalizedSave{}, bizerr.NewCode(CodeInvalid)
		}
	} else if !countryCallingCodePattern.MatchString(value.phoneCountryCode) || !phonePattern.MatchString(value.phone) {
		return normalizedSave{}, bizerr.NewCode(CodeInvalid)
	}
	return value, nil
}

func project(row contactRow) (Contact, error) {
	status := Status(row.StatusCode)
	if !contactNumberPattern.MatchString(row.ContactNo) || (status != StatusActive && status != StatusArchived) || row.Version < 1 || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return Contact{}, gerror.New("customer contact projection is inconsistent")
	}
	return Contact{ContactNo: row.ContactNo, DisplayName: row.DisplayName, Email: row.Email,
		EmailVerified: row.EmailVerifiedAt != nil, PhoneCountryCode: row.PhoneCountryCode, Phone: row.Phone,
		PhoneVerified: row.PhoneVerifiedAt != nil, PreferredLocale: row.PreferredLocale, Status: status,
		Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (s *serviceImpl) destinationDigest(kind, value string) string {
	if value == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.lookupKey)
	_, _ = mac.Write([]byte("customer-contact-" + kind + "-v1\x00" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

func digest(domain, value string) string {
	sum := sha256.Sum256([]byte(domain + "\x00" + value))
	return hex.EncodeToString(sum[:])
}

func newContactNo() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "CTC_" + hex.EncodeToString(raw[:]), nil
}

func safeSnapshot(item *Contact) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return map[string]any{"version": item.Version, "status": item.Status,
		"email_configured": item.Email != "", "email_verified": item.EmailVerified,
		"phone_configured": item.Phone != "", "phone_verified": item.PhoneVerified,
		"preferred_locale": item.PreferredLocale}
}

func safeRowSnapshot(item *contactRow) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return map[string]any{"version": item.Version, "status": item.StatusCode,
		"email_configured": item.Email != "", "email_verified": item.EmailVerifiedAt != nil,
		"phone_configured": item.Phone != "", "phone_verified": item.PhoneVerifiedAt != nil,
		"preferred_locale": item.PreferredLocale}
}

func mapResult(result *Contact, err error) (*Contact, error) {
	if err == nil && result != nil {
		return result, nil
	}
	if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeVersionConflict) || bizerr.Is(err, CodeIdempotencyConflict) || bizerr.Is(err, CodeUnavailable) {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeUnavailable)
}
