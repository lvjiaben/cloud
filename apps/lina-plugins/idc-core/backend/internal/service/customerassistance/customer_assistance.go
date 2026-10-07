// Package customerassistance owns short-lived, single-use customer impersonation sessions.
package customerassistance

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	authtoken "lina-core/pkg/plugin/capability/authcap/token"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PermissionCustomerAssist = "idc:admin:customer:assist"
	sessionDuration          = 15 * time.Minute
)

type StartInput struct {
	CustomerNo     string
	Reason         string
	TicketNo       string
	IdempotencyKey string
}

type Session struct {
	SessionNo   string
	CustomerNo  string
	Status      string
	AccessToken string
	StartedAt   time.Time
	ExpiresAt   time.Time
	EndedAt     *time.Time
}

type Snapshot struct {
	SessionNo        string
	CustomerNo       string
	CustomerType     string
	AccountStatus    string
	Verification     string
	DisplayName      string
	CountryCode      string
	PreferredLocale  string
	StartedAt        time.Time
	ExpiresAt        time.Time
	ReadOnly         bool
	ForbiddenActions []string
	AccessToken      string
}

type Service interface {
	Start(ctx context.Context, input StartInput) (*Session, error)
	End(ctx context.Context, sessionNo string) (*Session, error)
	Snapshot(ctx context.Context, accessToken string) (*Snapshot, error)
	Expire(ctx context.Context, limit int) (int, error)
}

type ExpiryWorker interface {
	Expire(ctx context.Context, limit int) (int, error)
}

type expiryWorker struct{ database gdb.DB }

func NewExpiryWorker(database gdb.DB) (ExpiryWorker, error) {
	if database == nil {
		return nil, gerror.New("customer assistance expiry requires database")
	}
	return &expiryWorker{database: database}, nil
}

func (w *expiryWorker) Expire(ctx context.Context, limit int) (int, error) {
	return expireSessions(ctx, w.database, limit)
}

type serviceImpl struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	audit         idccap.BusinessAuditWriter
	authToken     authtoken.Service
	tokenKey      []byte
}

func New(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, auditWriter idccap.BusinessAuditWriter, authToken authtoken.Service, tokenKey []byte) (Service, error) {
	if database == nil || bizCtx == nil || principalReader == nil || authorization == nil || auditWriter == nil || authToken == nil || len(tokenKey) != sha256.Size {
		return nil, gerror.New("customer assistance requires database, authorization, audit, and token key")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, principal: principalReader, authorization: authorization, audit: auditWriter, authToken: authToken, tokenKey: append([]byte(nil), tokenKey...)}, nil
}

func (s *serviceImpl) Start(ctx context.Context, input StartInput) (*Session, error) {
	staffID, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	input, err = normalizeStartInput(input)
	if err != nil {
		return nil, err
	}
	fingerprint := digest(input.CustomerNo + "\x00" + input.Reason + "\x00" + input.TicketNo)
	keyDigest := digest(input.IdempotencyKey)
	accessToken := s.accessToken(staffID, input, fingerprint)
	tokenDigest := digest(accessToken)
	result := &Session{CustomerNo: input.CustomerNo, Status: "active", AccessToken: accessToken}
	issuedBearer := ""
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.Exec(txCtx, `SELECT pg_advisory_xact_lock(761003, ?)`, staffID); lockErr != nil {
			return gerror.Wrap(lockErr, "serialize customer assistance staff session")
		}
		var existingRows []struct {
			SessionNo   string     `orm:"session_no"`
			CustomerNo  string     `orm:"customer_no"`
			Fingerprint string     `orm:"request_fingerprint"`
			Status      string     `orm:"status_code"`
			StartedAt   time.Time  `orm:"started_at"`
			ExpiresAt   time.Time  `orm:"expires_at"`
			EndedAt     *time.Time `orm:"ended_at"`
		}
		if scanErr := s.database.GetScan(txCtx, &existingRows, `SELECT session_no,customer_no,request_fingerprint,status_code,started_at,expires_at,ended_at FROM plugin_idc_core_customer_assistance_session WHERE tenant_id=0 AND staff_user_id=? AND idempotency_key_digest=? FOR UPDATE`, staffID, keyDigest); scanErr != nil {
			return gerror.Wrap(scanErr, "load customer assistance replay")
		}
		if len(existingRows) > 1 {
			return gerror.New("customer assistance idempotency key is ambiguous")
		}
		if len(existingRows) == 1 {
			existing := existingRows[0]
			if existing.Fingerprint != fingerprint || existing.CustomerNo != input.CustomerNo {
				return bizerr.NewCode(customer.CodeIdempotencyConflict)
			}
			result.SessionNo, result.CustomerNo, result.Status = existing.SessionNo, existing.CustomerNo, existing.Status
			result.StartedAt, result.ExpiresAt, result.EndedAt = existing.StartedAt, existing.ExpiresAt, existing.EndedAt
			return nil
		}
		var target struct {
			UserID int `orm:"sys_user_id"`
		}
		if existsErr := s.database.GetScan(txCtx, &target, `
SELECT binding.sys_user_id
FROM plugin_idc_core_customer_subject customer
JOIN plugin_idc_core_customer_identity_binding binding
  ON binding.tenant_id=customer.tenant_id AND binding.customer_no=customer.customer_no
JOIN plugin_idc_core_principal_audience audience
  ON audience.tenant_id=binding.tenant_id AND audience.sys_user_id=binding.sys_user_id
WHERE customer.tenant_id=0 AND customer.customer_no=? AND customer.status_code<>'anonymized'
  AND binding.status_code='active' AND audience.audience_code='customer' AND audience.status_code='active'
LIMIT 1`, input.CustomerNo); existsErr != nil || target.UserID <= 0 {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		nowValue, timeErr := s.database.GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return gerror.Wrap(timeErr, "read assistance database time")
		}
		result.StartedAt = nowValue.Time()
		result.ExpiresAt = result.StartedAt.Add(sessionDuration)
		hostToken, issueErr := s.authToken.IssueImpersonationToken(txCtx, authtoken.ImpersonationTokenIssueInput{
			ActingUserID: staffID, SubjectUserID: target.UserID,
			TenantID: 0, ExpiresInSeconds: int(sessionDuration / time.Second),
		})
		if issueErr != nil || hostToken == nil || hostToken.AccessToken == "" || hostToken.TokenID == "" || hostToken.SubjectUserID != target.UserID {
			if issueErr != nil {
				return gerror.Wrap(issueErr, "issue customer impersonation token")
			}
			return gerror.New("customer impersonation token is incomplete")
		}
		if hostToken.ExpiresAt.IsZero() || hostToken.ExpiresAt.Before(result.ExpiresAt.Add(-5*time.Second)) {
			return gerror.New("customer impersonation token lifetime is too short")
		}
		issuedBearer = hostToken.AccessToken
		encryptedBearer, encryptErr := s.encryptToken(hostToken.AccessToken)
		if encryptErr != nil {
			return encryptErr
		}
		var superseded []struct {
			SessionNo           string `orm:"session_no"`
			CustomerNo          string `orm:"customer_no"`
			HostTokenCiphertext string `orm:"host_token_ciphertext"`
		}
		if scanErr := s.database.GetScan(txCtx, &superseded, `UPDATE plugin_idc_core_customer_assistance_session SET status_code='superseded',ended_at=?,updated_at=? WHERE tenant_id=0 AND staff_user_id=? AND status_code='active' RETURNING session_no,customer_no,host_token_ciphertext`, result.StartedAt, result.StartedAt, staffID); scanErr != nil {
			return gerror.Wrap(scanErr, "supersede previous assistance session")
		}
		for _, previous := range superseded {
			if previous.HostTokenCiphertext != "" {
				bearer, decryptErr := s.decryptToken(previous.HostTokenCiphertext)
				if decryptErr != nil {
					return decryptErr
				}
				if revokeErr := s.authToken.RevokeImpersonationToken(txCtx, authtoken.ImpersonationTokenRevokeInput{BearerToken: bearer}); revokeErr != nil {
					return gerror.Wrap(revokeErr, "revoke superseded customer impersonation token")
				}
			}
			if eventErr := s.appendEvent(txCtx, previous.SessionNo, previous.CustomerNo, staffID, "superseded", result.StartedAt); eventErr != nil {
				return eventErr
			}
		}
		result.SessionNo, err = randomNumber("AST_")
		if err != nil {
			return err
		}
		if _, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_customer_assistance_session (tenant_id,session_no,customer_no,staff_user_id,reason,ticket_no,token_digest,host_token_id,host_token_ciphertext,idempotency_key_digest,request_fingerprint,status_code,started_at,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,?,'active',?,?,?,?)`, result.SessionNo, input.CustomerNo, staffID, input.Reason, input.TicketNo, tokenDigest, hostToken.TokenID, encryptedBearer, keyDigest, fingerprint, result.StartedAt, result.ExpiresAt, result.StartedAt, result.StartedAt); insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer assistance session")
		}
		if eventErr := s.appendEvent(txCtx, result.SessionNo, input.CustomerNo, staffID, "started", result.StartedAt); eventErr != nil {
			return eventErr
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "customer.assistance.start", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "customer.assistance.start", TargetType: "customer_assistance", TargetNo: result.SessionNo,
			CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_assistance_requested",
			RiskCode: "medium", RequiredPermission: PermissionCustomerAssist, CorrelationNo: result.SessionNo,
			BeforeSnapshot: map[string]any{"status": "absent"}, AfterSnapshot: map[string]any{"status": "active", "expires_at": result.ExpiresAt, "ticket_linked": input.TicketNo != ""},
		})
	})
	if err != nil {
		if issuedBearer != "" {
			_ = s.authToken.RevokeImpersonationToken(ctx, authtoken.ImpersonationTokenRevokeInput{BearerToken: issuedBearer})
		}
		return nil, wrap(err)
	}
	return result, nil
}

func normalizeStartInput(input StartInput) (StartInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.Reason = strings.TrimSpace(input.Reason)
	input.TicketNo = strings.TrimSpace(input.TicketNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validCustomerNo(input.CustomerNo) || !utf8.ValidString(input.Reason) || utf8.RuneCountInString(input.Reason) < 10 || utf8.RuneCountInString(input.Reason) > 512 ||
		!utf8.ValidString(input.TicketNo) || utf8.RuneCountInString(input.TicketNo) > 64 || !validIdempotencyKey(input.IdempotencyKey) {
		return StartInput{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	return input, nil
}

func (s *serviceImpl) End(ctx context.Context, sessionNo string) (*Session, error) {
	staffID, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	sessionNo = strings.TrimSpace(sessionNo)
	if !validNumber(sessionNo, "AST_") {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	result := &Session{SessionNo: sessionNo}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var row struct {
			CustomerNo          string     `orm:"customer_no"`
			Status              string     `orm:"status_code"`
			HostTokenCiphertext string     `orm:"host_token_ciphertext"`
			StartedAt           time.Time  `orm:"started_at"`
			ExpiresAt           time.Time  `orm:"expires_at"`
			EndedAt             *time.Time `orm:"ended_at"`
		}
		if scanErr := s.database.GetScan(txCtx, &row, `SELECT customer_no,status_code,host_token_ciphertext,started_at,expires_at,ended_at FROM plugin_idc_core_customer_assistance_session WHERE tenant_id=0 AND session_no=? AND staff_user_id=? FOR UPDATE`, sessionNo, staffID); scanErr != nil || row.CustomerNo == "" {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		result.CustomerNo, result.Status, result.StartedAt, result.ExpiresAt, result.EndedAt = row.CustomerNo, row.Status, row.StartedAt, row.ExpiresAt, row.EndedAt
		if row.Status != "active" {
			return nil
		}
		if row.HostTokenCiphertext != "" {
			bearer, decryptErr := s.decryptToken(row.HostTokenCiphertext)
			if decryptErr != nil {
				return decryptErr
			}
			if revokeErr := s.authToken.RevokeImpersonationToken(txCtx, authtoken.ImpersonationTokenRevokeInput{BearerToken: bearer}); revokeErr != nil {
				return gerror.Wrap(revokeErr, "revoke customer impersonation token")
			}
		}
		nowValue, timeErr := s.database.GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return timeErr
		}
		endedAt := nowValue.Time()
		result.Status, result.EndedAt = "ended", &endedAt
		if _, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_customer_assistance_session SET status_code='ended',ended_at=?,updated_at=? WHERE tenant_id=0 AND session_no=? AND status_code='active'`, endedAt, endedAt, sessionNo); updateErr != nil {
			return updateErr
		}
		if eventErr := s.appendEvent(txCtx, sessionNo, row.CustomerNo, staffID, "ended", endedAt); eventErr != nil {
			return eventErr
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "customer.assistance.end", IdempotencyKey: "assistance-end-" + sessionNo,
			OperationCode: "customer.assistance.end", TargetType: "customer_assistance", TargetNo: sessionNo,
			CustomerNo: row.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_assistance_ended",
			RiskCode: "low", RequiredPermission: PermissionCustomerAssist, CorrelationNo: sessionNo,
			BeforeSnapshot: map[string]any{"status": "active"}, AfterSnapshot: map[string]any{"status": "ended"},
		})
	})
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *serviceImpl) Snapshot(ctx context.Context, accessToken string) (*Snapshot, error) {
	accessToken = strings.TrimSpace(accessToken)
	if len(accessToken) < 32 || len(accessToken) > 128 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	result := &Snapshot{ReadOnly: false, ForbiddenActions: []string{}}
	expiredDuringRead := false
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var row struct {
			SessionNo           string     `orm:"session_no"`
			CustomerNo          string     `orm:"customer_no"`
			StaffID             int        `orm:"staff_user_id"`
			Status              string     `orm:"status_code"`
			HostTokenCiphertext string     `orm:"host_token_ciphertext"`
			ExchangedAt         *time.Time `orm:"exchanged_at"`
			StartedAt           time.Time  `orm:"started_at"`
			ExpiresAt           time.Time  `orm:"expires_at"`
		}
		if scanErr := s.database.GetScan(txCtx, &row, `SELECT session_no,customer_no,staff_user_id,status_code,host_token_ciphertext,exchanged_at,started_at,expires_at FROM plugin_idc_core_customer_assistance_session WHERE tenant_id=0 AND token_digest=? FOR UPDATE`, digest(accessToken)); scanErr != nil || row.SessionNo == "" {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		nowValue, timeErr := s.database.GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return timeErr
		}
		now := nowValue.Time()
		if row.Status == "active" && !now.Before(row.ExpiresAt) {
			if _, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_customer_assistance_session SET status_code='expired',ended_at=?,updated_at=? WHERE tenant_id=0 AND session_no=? AND status_code='active'`, now, now, row.SessionNo); updateErr != nil {
				return updateErr
			}
			if eventErr := s.appendEvent(txCtx, row.SessionNo, row.CustomerNo, row.StaffID, "expired", now); eventErr != nil {
				return eventErr
			}
			expiredDuringRead = true
			return nil
		}
		if row.Status != "active" || row.ExchangedAt != nil || row.HostTokenCiphertext == "" {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		bearer, decryptErr := s.decryptToken(row.HostTokenCiphertext)
		if decryptErr != nil {
			return decryptErr
		}
		updated, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_customer_assistance_session SET exchanged_at=?,updated_at=? WHERE tenant_id=0 AND session_no=? AND status_code='active' AND exchanged_at IS NULL`, now, now, row.SessionNo)
		if updateErr != nil {
			return updateErr
		}
		changed, updateErr := updated.RowsAffected()
		if updateErr != nil || changed != 1 {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		if eventErr := s.appendEvent(txCtx, row.SessionNo, row.CustomerNo, row.StaffID, "exchanged", now); eventErr != nil {
			return eventErr
		}
		result.SessionNo, result.CustomerNo, result.StartedAt, result.ExpiresAt = row.SessionNo, row.CustomerNo, row.StartedAt, row.ExpiresAt
		result.AccessToken = bearer
		return nil
	})
	if err != nil {
		return nil, wrap(err)
	}
	if expiredDuringRead {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return result, nil
}

// Expire records authoritative database-time expiry even when no browser
// attempts another read after the fifteen-minute window closes.
func (s *serviceImpl) Expire(ctx context.Context, limit int) (int, error) {
	return expireSessions(ctx, s.database, limit)
}

func expireSessions(ctx context.Context, database gdb.DB, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		return 0, gerror.New("customer assistance expiry limit is invalid")
	}
	expiredCount := 0
	err := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var rows []struct {
			SessionNo  string `orm:"session_no"`
			CustomerNo string `orm:"customer_no"`
			StaffID    int    `orm:"staff_user_id"`
		}
		if scanErr := database.GetScan(txCtx, &rows, `SELECT session_no,customer_no,staff_user_id FROM plugin_idc_core_customer_assistance_session WHERE tenant_id=0 AND status_code='active' AND expires_at<=CURRENT_TIMESTAMP ORDER BY expires_at,id LIMIT ? FOR UPDATE SKIP LOCKED`, limit); scanErr != nil {
			return gerror.Wrap(scanErr, "claim expired customer assistance sessions")
		}
		if len(rows) == 0 {
			return nil
		}
		nowValue, timeErr := database.GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return gerror.Wrap(timeErr, "read assistance expiry database time")
		}
		now := nowValue.Time()
		for _, row := range rows {
			result, updateErr := database.Exec(txCtx, `UPDATE plugin_idc_core_customer_assistance_session SET status_code='expired',ended_at=?,updated_at=? WHERE tenant_id=0 AND session_no=? AND status_code='active'`, now, now, row.SessionNo)
			if updateErr != nil {
				return gerror.Wrap(updateErr, "expire customer assistance session")
			}
			changed, updateErr := result.RowsAffected()
			if updateErr != nil {
				return gerror.Wrap(updateErr, "read customer assistance expiry result")
			}
			if changed == 0 {
				continue
			}
			if eventErr := appendAssistanceEvent(txCtx, database, row.SessionNo, row.CustomerNo, row.StaffID, "expired", now); eventErr != nil {
				return eventErr
			}
			expiredCount++
		}
		return nil
	})
	if err != nil {
		return 0, gerror.Wrap(err, "expire customer assistance sessions")
	}
	return expiredCount, nil
}

func (s *serviceImpl) authorize(ctx context.Context) (int, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	keys := []authz.PermissionKey{authz.PermissionKey(PermissionCustomerAssist), authz.PermissionKey(idccap.HostPermissionWildcard)}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, customer.CodeNotAccessible)
	}
	if grants[keys[0]] || grants[keys[1]] {
		return current.UserID, nil
	}
	admin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || !admin {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return current.UserID, nil
}

func (s *serviceImpl) appendEvent(ctx context.Context, sessionNo, customerNo string, staffID int, eventType string, occurredAt time.Time) error {
	return appendAssistanceEvent(ctx, s.database, sessionNo, customerNo, staffID, eventType, occurredAt)
}

func appendAssistanceEvent(ctx context.Context, database gdb.DB, sessionNo, customerNo string, staffID int, eventType string, occurredAt time.Time) error {
	eventNo, err := randomNumber("ASE_")
	if err != nil {
		return err
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_assistance_event (tenant_id,event_no,session_no,customer_no,staff_user_id,event_type,occurred_at,created_at) VALUES (0,?,?,?,?,?,?,?)`, eventNo, sessionNo, customerNo, staffID, eventType, occurredAt, occurredAt)
	return gerror.Wrap(err, "append customer assistance event")
}

func (s *serviceImpl) accessToken(staffID int, input StartInput, fingerprint string) string {
	mac := hmac.New(sha256.New, s.tokenKey)
	_, _ = mac.Write([]byte(strconv.Itoa(staffID) + "\x00" + input.CustomerNo + "\x00" + input.IdempotencyKey + "\x00" + fingerprint))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *serviceImpl) encryptToken(value string) (string, error) {
	block, err := aes.NewCipher(s.tokenKey)
	if err != nil {
		return "", gerror.Wrap(err, "create customer impersonation cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", gerror.Wrap(err, "create customer impersonation token seal")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", gerror.Wrap(err, "generate customer impersonation token nonce")
	}
	sealed := aead.Seal(nil, nonce, []byte(value), nil)
	payload := make([]byte, 0, len(nonce)+len(sealed))
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (s *serviceImpl) decryptToken(value string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", gerror.Wrap(err, "decode customer impersonation token")
	}
	block, err := aes.NewCipher(s.tokenKey)
	if err != nil {
		return "", gerror.Wrap(err, "create customer impersonation cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", gerror.Wrap(err, "create customer impersonation token seal")
	}
	if len(raw) <= aead.NonceSize() {
		return "", gerror.New("customer impersonation token payload is invalid")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil {
		return "", gerror.Wrap(err, "open customer impersonation token")
	}
	if len(plain) == 0 {
		return "", gerror.New("customer impersonation token is empty")
	}
	return string(plain), nil
}

func randomNumber(prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", gerror.Wrap(err, "generate customer assistance number")
	}
	return prefix + hex.EncodeToString(value), nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func validCustomerNo(value string) bool { return validNumber(value, "CUS_") }
func validNumber(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+32 {
		return false
	}
	_, err := hex.DecodeString(value[len(prefix):])
	return err == nil
}
func validIdempotencyKey(value string) bool {
	return len(value) >= 12 && len(value) <= 128 && utf8.ValidString(value)
}
func wrap(err error) error {
	if bizerr.Is(err, customer.CodeNotAccessible) || bizerr.Is(err, customer.CodeProfileInvalid) || bizerr.Is(err, customer.CodeIdempotencyConflict) {
		return err
	}
	return bizerr.WrapCode(err, customer.CodeUnavailable)
}
