package customeraddress

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	CodeInvalid             = bizerr.MustDefine("IDC_CUSTOMER_ADDRESS_INVALID", "The customer address request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible       = bizerr.MustDefine("IDC_CUSTOMER_ADDRESS_NOT_ACCESSIBLE", "The customer address is not accessible", gcode.CodeNotAuthorized)
	CodeVersionConflict     = bizerr.MustDefine("IDC_CUSTOMER_ADDRESS_VERSION_CONFLICT", "The customer address has changed", gcode.CodeInvalidOperation)
	CodeIdempotencyConflict = bizerr.MustDefine("IDC_CUSTOMER_ADDRESS_IDEMPOTENCY_CONFLICT", "The customer address request conflicts with an earlier request", gcode.CodeInvalidOperation)
	CodeUnavailable         = bizerr.MustDefine("IDC_CUSTOMER_ADDRESS_UNAVAILABLE", "The customer address service is temporarily unavailable", gcode.CodeInternalError)
)

const (
	scopeSave    = "customer.address.save"
	scopeArchive = "customer.address.archive"
)

var (
	addressNumberPattern = regexp.MustCompile(`^ADR_[0-9a-f]{32}$`)
	countryPattern       = regexp.MustCompile(`^[A-Z]{2}$`)
)

type serviceImpl struct {
	database  gdb.DB
	access    *customer.AccessGuard
	audit     audit.Writer
	lookupKey []byte
}

func New(database gdb.DB, access *customer.AccessGuard, auditWriter audit.Writer, lookupKey []byte) (Service, error) {
	if database == nil || access == nil || auditWriter == nil || len(lookupKey) < 32 {
		return nil, gerror.New("customer address service requires all dependencies")
	}
	return &serviceImpl{database: database, access: access, audit: auditWriter, lookupKey: append([]byte(nil), lookupKey...)}, nil
}

type addressRow struct {
	AddressNo, AddressTypeCode, Label, RecipientName, RecipientPhone, CountryCode string
	Province, City, District, AddressLine1, AddressLine2, PostalCode              string
	IsDefault                                                                     bool
	StatusCode                                                                    string
	Version                                                                       int64
	CreatedAt, UpdatedAt                                                          time.Time
}

func (s *serviceImpl) List(ctx context.Context) ([]Address, error) {
	audience, err := s.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []addressRow
	if err = s.database.GetScan(ctx, &rows, `SELECT address_no,address_type_code,label,recipient_name,recipient_phone,country_code,province,city,district,address_line1,address_line2,postal_code,is_default,status_code,version,created_at,updated_at FROM plugin_idc_core_customer_address WHERE tenant_id=0 AND customer_no=? ORDER BY CASE status_code WHEN 'active' THEN 0 ELSE 1 END,address_type_code,is_default DESC,id LIMIT 100`, audience.CustomerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	items := make([]Address, 0, len(rows))
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
	SaveInput
	addressNo, idempotencyKey string
}

func (s *serviceImpl) Save(ctx context.Context, input SaveInput) (*Address, error) {
	audience, err := s.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeSave(input)
	if err != nil {
		return nil, err
	}
	fingerprint := s.digest("customer-address-save-fingerprint-v1", strings.Join([]string{
		normalized.addressNo, string(normalized.Type), normalized.Label, normalized.RecipientName,
		normalized.RecipientPhone, normalized.CountryCode, normalized.Province, normalized.City,
		normalized.District, normalized.AddressLine1, normalized.AddressLine2, normalized.PostalCode,
		strconv.FormatBool(normalized.Default), strconv.FormatInt(normalized.ExpectedVersion, 10),
	}, "\x00"))
	var result *Address
	err = s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		reservation, reserveErr := s.reserve(tx, scopeSave, audience.UserID, audience.CustomerNo, normalized.idempotencyKey, fingerprint)
		if reserveErr != nil {
			return reserveErr
		}
		if reservation.replay {
			result, reserveErr = loadReplay(tx, reservation, audience.CustomerNo)
			return reserveErr
		}
		if normalized.addressNo == "" {
			result, reserveErr = s.create(tx, audience.UserID, audience.CustomerNo, normalized)
		} else {
			result, reserveErr = s.update(tx, audience.UserID, audience.CustomerNo, normalized)
		}
		if reserveErr != nil {
			return reserveErr
		}
		if _, reserveErr = s.audit.AppendTransition(txCtx, audit.TransitionInput{Fields: audit.Fields{
			IdempotencyScope: scopeSave, IdempotencyKey: normalized.idempotencyKey, OperationCode: scopeSave,
			TargetType: "customer_address", TargetNo: result.AddressNo, CustomerNo: audience.CustomerNo, Result: audit.ResultSucceeded, Risk: audit.RiskLow,
		}, BeforeSnapshot: map[string]any{"version": normalized.ExpectedVersion}, AfterSnapshot: safeSnapshot(result)}); reserveErr != nil {
			return reserveErr
		}
		return complete(tx, reservation.id, result.AddressNo, result.Version)
	})
	return mapResult(result, err)
}

func (s *serviceImpl) Archive(ctx context.Context, input ArchiveInput) (*Address, error) {
	audience, err := s.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	addressNo, key := strings.TrimSpace(input.AddressNo), strings.TrimSpace(input.IdempotencyKey)
	if !addressNumberPattern.MatchString(addressNo) || input.ExpectedVersion < 1 || len(key) < 16 || len(key) > 128 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	fingerprint := s.digest("customer-address-archive-fingerprint-v1", addressNo+"\x00"+strconv.FormatInt(input.ExpectedVersion, 10))
	var result *Address
	err = s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		reservation, reserveErr := s.reserve(tx, scopeArchive, audience.UserID, audience.CustomerNo, key, fingerprint)
		if reserveErr != nil {
			return reserveErr
		}
		if reservation.replay {
			result, reserveErr = loadReplay(tx, reservation, audience.CustomerNo)
			return reserveErr
		}
		before, loadErr := load(tx, audience.CustomerNo, addressNo, true)
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
		updated, updateErr := tx.Exec(`UPDATE plugin_idc_core_customer_address SET status_code='archived',is_default=FALSE,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND address_no=? AND status_code='active' AND version=?`, audience.UserID, now, audience.CustomerNo, addressNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := updated.RowsAffected(); affectedErr != nil || affected != 1 {
			return bizerr.WrapCode(affectedErr, CodeVersionConflict)
		}
		after, loadErr := load(tx, audience.CustomerNo, addressNo, false)
		if loadErr != nil || after == nil {
			return gerror.Wrap(loadErr, "load archived customer address")
		}
		projected, projectErr := project(*after)
		if projectErr != nil {
			return projectErr
		}
		result = &projected
		if _, updateErr = s.audit.AppendTransition(txCtx, audit.TransitionInput{Fields: audit.Fields{
			IdempotencyScope: scopeArchive, IdempotencyKey: key, OperationCode: scopeArchive,
			TargetType: "customer_address", TargetNo: addressNo, CustomerNo: audience.CustomerNo, Result: audit.ResultSucceeded, Risk: audit.RiskLow,
		}, BeforeSnapshot: safeRowSnapshot(before), AfterSnapshot: safeSnapshot(result)}); updateErr != nil {
			return updateErr
		}
		return complete(tx, reservation.id, result.AddressNo, result.Version)
	})
	return mapResult(result, err)
}

func (s *serviceImpl) create(tx gdb.TX, userID int, customerNo string, input normalizedSave) (*Address, error) {
	addressNo, err := newAddressNo()
	if err != nil {
		return nil, err
	}
	now, err := databaseTime(tx)
	if err != nil {
		return nil, err
	}
	if input.Default {
		if _, err = tx.Exec(`UPDATE plugin_idc_core_customer_address SET is_default=FALSE,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND address_type_code=? AND status_code='active' AND is_default=TRUE`, userID, now, customerNo, input.Type); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(`INSERT INTO plugin_idc_core_customer_address (tenant_id,address_no,customer_no,address_type_code,label,recipient_name,recipient_phone,country_code,province,city,district,address_line1,address_line2,postal_code,is_default,status_code,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'active',1,?,?,?,?)`,
		addressNo, customerNo, input.Type, input.Label, input.RecipientName, input.RecipientPhone, input.CountryCode,
		input.Province, input.City, input.District, input.AddressLine1, input.AddressLine2, input.PostalCode,
		input.Default, userID, userID, now, now)
	if err != nil {
		return nil, err
	}
	return loadProjected(tx, customerNo, addressNo)
}

func (s *serviceImpl) update(tx gdb.TX, userID int, customerNo string, input normalizedSave) (*Address, error) {
	before, err := load(tx, customerNo, input.addressNo, true)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if before == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if before.StatusCode != string(StatusActive) || before.Version != input.ExpectedVersion {
		return nil, bizerr.NewCode(CodeVersionConflict)
	}
	now, err := databaseTime(tx)
	if err != nil {
		return nil, err
	}
	if input.Default {
		if _, err = tx.Exec(`UPDATE plugin_idc_core_customer_address SET is_default=FALSE,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND address_type_code=? AND address_no<>? AND status_code='active' AND is_default=TRUE`, userID, now, customerNo, input.Type, input.addressNo); err != nil {
			return nil, err
		}
	}
	updated, err := tx.Exec(`UPDATE plugin_idc_core_customer_address SET address_type_code=?,label=?,recipient_name=?,recipient_phone=?,country_code=?,province=?,city=?,district=?,address_line1=?,address_line2=?,postal_code=?,is_default=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND address_no=? AND status_code='active' AND version=?`,
		input.Type, input.Label, input.RecipientName, input.RecipientPhone, input.CountryCode, input.Province, input.City,
		input.District, input.AddressLine1, input.AddressLine2, input.PostalCode, input.Default, userID, now,
		customerNo, input.addressNo, input.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	if affected, affectedErr := updated.RowsAffected(); affectedErr != nil || affected != 1 {
		return nil, bizerr.WrapCode(affectedErr, CodeVersionConflict)
	}
	return loadProjected(tx, customerNo, input.addressNo)
}

type reservation struct {
	id       int64
	replay   bool
	resource string
	version  int64
}

func (s *serviceImpl) reserve(tx gdb.TX, scope string, userID int, customerNo, key, fingerprint string) (reservation, error) {
	caller := s.digest("customer-address-caller-v1", customerNo+"\x00"+strconv.Itoa(userID))
	keyDigest := s.digest("customer-address-key-v1", key)
	now, err := databaseTime(tx)
	if err != nil {
		return reservation{}, err
	}
	insertedResult, err := tx.Exec(`INSERT INTO plugin_idc_core_idempotency_record (tenant_id,scope,caller_scope_digest,idempotency_key_digest,request_fingerprint,status_code,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,'in_progress',?,?,?) ON CONFLICT (tenant_id,scope,caller_scope_digest,idempotency_key_digest) DO NOTHING`, scope, caller, keyDigest, fingerprint, now.Add(30*24*time.Hour), now, now)
	if err != nil {
		return reservation{}, err
	}
	inserted, err := insertedResult.RowsAffected()
	if err != nil {
		return reservation{}, err
	}
	type storedRow struct {
		ID                 int64
		RequestFingerprint string
		StatusCode         string
		ResourceNo         string
		ResponseSnapshot   string
	}
	var stored *storedRow
	if err = tx.GetScan(&stored, `SELECT id,request_fingerprint,status_code,resource_no,response_snapshot::text FROM plugin_idc_core_idempotency_record WHERE tenant_id=0 AND scope=? AND caller_scope_digest=? AND idempotency_key_digest=? FOR UPDATE`, scope, caller, keyDigest); err != nil || stored == nil {
		return reservation{}, gerror.Wrap(err, "load customer address idempotency reservation")
	}
	if stored.RequestFingerprint != fingerprint {
		return reservation{}, bizerr.NewCode(CodeIdempotencyConflict)
	}
	if inserted == 1 {
		return reservation{id: stored.ID}, nil
	}
	if stored.StatusCode != "succeeded" || !addressNumberPattern.MatchString(stored.ResourceNo) {
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

func complete(tx gdb.TX, id int64, addressNo string, version int64) error {
	snapshot, err := json.Marshal(map[string]any{"version": version})
	if err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE plugin_idc_core_idempotency_record SET status_code='succeeded',resource_type='customer_address',resource_no=?,response_status_code=200,response_snapshot=?::jsonb,completed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND id=? AND status_code='in_progress'`, addressNo, string(snapshot), id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.Wrap(err, "complete customer address idempotency reservation")
	}
	return nil
}

func loadReplay(tx gdb.TX, value reservation, customerNo string) (*Address, error) {
	item, err := loadProjected(tx, customerNo, value.resource)
	if err != nil || item == nil || item.Version < value.version {
		return nil, gerror.Wrap(err, "load customer address replay")
	}
	return item, nil
}

func loadProjected(tx gdb.TX, customerNo, addressNo string) (*Address, error) {
	row, err := load(tx, customerNo, addressNo, false)
	if err != nil || row == nil {
		return nil, gerror.Wrap(err, "load customer address")
	}
	item, err := project(*row)
	return &item, err
}

func load(tx gdb.TX, customerNo, addressNo string, lock bool) (*addressRow, error) {
	query := `SELECT address_no,address_type_code,label,recipient_name,recipient_phone,country_code,province,city,district,address_line1,address_line2,postal_code,is_default,status_code,version,created_at,updated_at FROM plugin_idc_core_customer_address WHERE tenant_id=0 AND customer_no=? AND address_no=?`
	if lock {
		query += ` FOR UPDATE`
	}
	var row *addressRow
	err := tx.GetScan(&row, query, customerNo, addressNo)
	return row, err
}

func normalizeSave(input SaveInput) (normalizedSave, error) {
	input.AddressNo, input.Label = strings.TrimSpace(input.AddressNo), strings.TrimSpace(input.Label)
	input.RecipientName, input.RecipientPhone = strings.TrimSpace(input.RecipientName), strings.TrimSpace(input.RecipientPhone)
	input.CountryCode = strings.ToUpper(strings.TrimSpace(input.CountryCode))
	input.Province, input.City, input.District = strings.TrimSpace(input.Province), strings.TrimSpace(input.City), strings.TrimSpace(input.District)
	input.AddressLine1, input.AddressLine2, input.PostalCode = strings.TrimSpace(input.AddressLine1), strings.TrimSpace(input.AddressLine2), strings.TrimSpace(input.PostalCode)
	key := strings.TrimSpace(input.IdempotencyKey)
	validType := input.Type == TypeContact || input.Type == TypeBilling || input.Type == TypeRegistered || input.Type == TypeService
	if (input.AddressNo != "" && !addressNumberPattern.MatchString(input.AddressNo)) || !validType || !countryPattern.MatchString(input.CountryCode) ||
		(input.AddressNo == "" && input.ExpectedVersion != 0) || (input.AddressNo != "" && input.ExpectedVersion < 1) ||
		len(input.Label) > 128 || len(input.RecipientName) > 256 || len(input.RecipientPhone) > 32 || len(input.Province) > 128 || len(input.City) > 128 || len(input.District) > 128 ||
		len(input.AddressLine1) < 1 || len(input.AddressLine1) > 512 || len(input.AddressLine2) > 512 || len(input.PostalCode) > 32 || len(key) < 16 || len(key) > 128 {
		return normalizedSave{}, bizerr.NewCode(CodeInvalid)
	}
	return normalizedSave{SaveInput: input, addressNo: input.AddressNo, idempotencyKey: key}, nil
}

func project(row addressRow) (Address, error) {
	item := Address{AddressNo: row.AddressNo, Type: Type(row.AddressTypeCode), Label: row.Label, RecipientName: row.RecipientName,
		RecipientPhone: row.RecipientPhone, CountryCode: row.CountryCode, Province: row.Province, City: row.City,
		District: row.District, AddressLine1: row.AddressLine1, AddressLine2: row.AddressLine2, PostalCode: row.PostalCode,
		Default: row.IsDefault, Status: Status(row.StatusCode), Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if !addressNumberPattern.MatchString(item.AddressNo) || (item.Status != StatusActive && item.Status != StatusArchived) || item.Version < 1 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return Address{}, gerror.New("customer address projection is inconsistent")
	}
	return item, nil
}

func (s *serviceImpl) digest(domain, value string) string {
	hash := hmac.New(sha256.New, s.lookupKey)
	_, _ = hash.Write([]byte(domain + "\x00" + value))
	return hex.EncodeToString(hash.Sum(nil))
}

func newAddressNo() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "ADR_" + hex.EncodeToString(raw[:]), nil
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

func safeSnapshot(item *Address) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return map[string]any{"address_no": item.AddressNo, "type": item.Type, "status": item.Status, "default": item.Default, "version": item.Version}
}

func safeRowSnapshot(row *addressRow) map[string]any {
	if row == nil {
		return map[string]any{}
	}
	return map[string]any{"address_no": row.AddressNo, "type": row.AddressTypeCode, "status": row.StatusCode, "default": row.IsDefault, "version": row.Version}
}

func mapResult(result *Address, err error) (*Address, error) {
	if err == nil {
		return result, nil
	}
	if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeNotAccessible) ||
		bizerr.Is(err, CodeVersionConflict) || bizerr.Is(err, CodeIdempotencyConflict) ||
		bizerr.Is(err, CodeUnavailable) {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeUnavailable)
}
