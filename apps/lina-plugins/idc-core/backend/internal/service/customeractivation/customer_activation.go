// Package customeractivation owns admin-created pending customers and their
// short-lived, single-use activation invitations.
package customeractivation

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/statusflag"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PermissionCustomerCreate = "idc:admin:customer:create"
	PermissionCustomerUpdate = "idc:admin:customer:update"
	invitationDuration       = 24 * time.Hour
)

var (
	activationCustomerNoPattern = regexp.MustCompile(`^CUS_[0-9a-f]{32}$`)
	activationCountryPattern    = regexp.MustCompile(`^[A-Z]{2}$`)
	activationCallingPattern    = regexp.MustCompile(`^\+[1-9][0-9]{0,6}$`)
	activationPhonePattern      = regexp.MustCompile(`^[0-9]{6,20}$`)
)

type CreateInput struct {
	CustomerType         string
	DisplayName          string
	CountryCode          string
	PreferredLocale      string
	Email                string
	PhoneCountryCode     string
	Phone                string
	BindExistingIdentity bool
	TargetUserID         int
	Reason               string
	IdempotencyKey       string
}

type ResendInput struct {
	CustomerNo     string
	Reason         string
	IdempotencyKey string
}

// DirectCreateInput carries one administrator-provided login and initial password.
type DirectCreateInput struct {
	CustomerType     string
	DisplayName      string
	CountryCode      string
	PreferredLocale  string
	Email            string
	PhoneCountryCode string
	Phone            string
	InitialPassword  string
	Reason           string
	IdempotencyKey   string
}

// PasswordResetInput carries one staff-confirmed customer password reset.
type PasswordResetInput struct {
	CustomerNo     string
	NewPassword    string
	Reason         string
	IdempotencyKey string
}

// DirectAccount is the non-secret result of direct customer creation.
type DirectAccount struct {
	CustomerNo string
	UserID     int
	Status     string
	Reused     bool
}

type Invitation struct {
	InvitationNo          string
	CustomerNo            string
	CustomerType          string
	DisplayName           string
	DestinationHint       string
	TargetUserID          int
	ExistingIdentityBound bool
	Status                string
	ExpiresAt             time.Time
	Reused                bool
}

type Activation struct {
	CustomerNo   string
	CustomerType string
	Status       string
	Reused       bool
}

type Service interface {
	ChangeInitialPassword(context.Context, string) error
	Create(context.Context, CreateInput) (*Invitation, error)
	CreateForChannel(context.Context, string, CreateInput) (*Invitation, error)
	CreateDirect(context.Context, DirectCreateInput) (*DirectAccount, error)
	ResetPassword(context.Context, PasswordResetInput) error
	InitialPasswordChangeRequired(context.Context) (bool, error)
	Resend(context.Context, ResendInput) (*Invitation, error)
	Redeem(context.Context, string) (*Activation, error)
}

// InitialPasswordChangeRequired reports the direct-created customer's current gate.
func (s *serviceImpl) InitialPasswordChangeRequired(ctx context.Context) (bool, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID > 0 && current.TenantID == 0 && current.IsImpersonation &&
		current.ActingUserID > 0 && current.ActingUserID != current.UserID && !current.ActingAsTenant {
		return false, nil
	}
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant {
		return false, bizerr.NewCode(customer.CodeNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return false, bizerr.NewCode(customer.CodeNotAccessible)
	}
	value, err := s.database.GetValue(ctx, `SELECT must_change_password FROM plugin_idc_core_customer_identity_binding WHERE tenant_id=0 AND sys_user_id=? AND customer_no=? AND status_code='active'`, current.UserID, audience.CustomerNo)
	if err != nil || value.IsNil() {
		if err != nil {
			return false, bizerr.WrapCode(err, customer.CodeUnavailable)
		}
		return false, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return value.Bool(), nil
}

// ChangeInitialPassword replaces the host password before clearing the plugin gate.
func (s *serviceImpl) ChangeInitialPassword(ctx context.Context, newPassword string) error {
	required, err := s.InitialPasswordChangeRequired(ctx)
	if err != nil {
		return err
	}
	if !required {
		return nil
	}
	newPassword = strings.TrimSpace(newPassword)
	if utf8.RuneCountInString(newPassword) < 8 || utf8.RuneCountInString(newPassword) > 128 {
		return bizerr.NewCode(customer.CodeProfileInvalid)
	}
	current := s.bizCtx.Current(ctx)
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	// The customer frontend has already saved the password through the host's
	// current-user profile endpoint. This plugin endpoint owns only the IDC
	// first-login gate and its business audit; it must not reach into host user
	// storage or use the staff-oriented reset-password capability for self-service.
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, txErr := dbNow(s.database, ctx)
		if txErr != nil {
			return txErr
		}
		result, txErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_customer_identity_binding SET must_change_password=FALSE,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND sys_user_id=? AND customer_no=? AND status_code='active' AND must_change_password=TRUE`, current.UserID, now, current.UserID, audience.CustomerNo)
		if txErr != nil {
			return txErr
		}
		affected, txErr := result.RowsAffected()
		if txErr != nil || affected != 1 {
			return bizerr.NewCode(customer.CodeUnavailable)
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "customer.initial_password.change", IdempotencyKey: audience.CustomerNo + ":" + strconv.FormatInt(now.UnixMilli(), 10), OperationCode: "customer.initial_password.change", TargetType: "customer", TargetNo: audience.CustomerNo, CustomerNo: audience.CustomerNo, ResultCode: "succeeded", ReasonCode: "first_login", RiskCode: "medium", CorrelationNo: audience.CustomerNo, BeforeSnapshot: map[string]any{"must_change_password": true}, AfterSnapshot: map[string]any{"must_change_password": false}})
	})
	if err != nil {
		return bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return nil
}

// CreateDirect creates an active customer and host login in the shared database transaction.
func (s *serviceImpl) CreateDirect(ctx context.Context, input DirectCreateInput) (*DirectAccount, error) {
	staffID, err := s.authorizeStaff(ctx, PermissionCustomerCreate)
	if err != nil {
		return nil, err
	}
	password := strings.TrimSpace(input.InitialPassword)
	input.InitialPassword = ""
	if utf8.RuneCountInString(password) < 8 || utf8.RuneCountInString(password) > 128 {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	value, err := normalizeDirectCreate(CreateInput{
		CustomerType: input.CustomerType, DisplayName: input.DisplayName, CountryCode: input.CountryCode,
		PreferredLocale: input.PreferredLocale, Email: input.Email, PhoneCountryCode: input.PhoneCountryCode,
		Phone: input.Phone, Reason: input.Reason, IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	requestDigest := digest(value.IdempotencyKey)
	requestFingerprint := value.fingerprint
	var result *DirectAccount
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var replay *struct {
			CustomerNo         string `orm:"customer_no"`
			SysUserID          int    `orm:"sys_user_id"`
			RequestFingerprint string `orm:"request_fingerprint"`
		}
		if txErr := s.database.GetScan(ctx, &replay, `SELECT customer_no,sys_user_id,request_fingerprint FROM plugin_idc_core_customer_identity_binding WHERE tenant_id=0 AND created_by=? AND idempotency_key_digest=? FOR UPDATE`, staffID, requestDigest); txErr != nil {
			return txErr
		}
		if replay != nil {
			if replay.RequestFingerprint != requestFingerprint {
				return bizerr.NewCode(customer.CodeIdempotencyConflict)
			}
			result = &DirectAccount{CustomerNo: replay.CustomerNo, UserID: replay.SysUserID, Status: "active", Reused: true}
			return nil
		}

		emailDigest, phoneDigest := "", ""
		if value.Email != "" {
			emailDigest = s.destinationDigest("email", value.Email)
		}
		if value.Phone != "" {
			phoneDigest = s.destinationDigest("sms", value.PhoneCountryCode+value.Phone)
		}
		duplicate, txErr := s.database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_customer_contact WHERE tenant_id=0 AND status_code='active' AND ((email_lookup_digest<>'' AND email_lookup_digest=?) OR (phone_lookup_digest<>'' AND phone_lookup_digest=?))`, emailDigest, phoneDigest)
		if txErr != nil {
			return txErr
		}
		if duplicate.Int() != 0 {
			return duplicateContactError(value.contactKind)
		}

		// Both the host profile and login projection use the same complete
		// number; the customer contact keeps its separate calling-code fields.
		var (
			loginName  = directLoginName(value)
			loginPhone string
			roleID     int
			enabled    = statusflag.EnabledValue
		)
		if value.Phone != "" {
			loginPhone = value.PhoneCountryCode + value.Phone
		}

		roleID, txErr = s.loginRecorder.DefaultRoleID(ctx)
		if txErr != nil {
			return txErr
		}
		createdUserID, txErr := s.users.Create(ctx, usercap.CreateInput{
			Username: loginName, Password: password, Nickname: value.DisplayName, RoleIDs: []int{roleID},
			Email: value.Email, Phone: loginPhone, Status: &enabled, Remark: "IDC customer created directly by an administrator",
		})
		if txErr != nil {
			return txErr
		}
		userID, txErr := strconv.Atoi(string(createdUserID))
		if txErr != nil || userID <= 0 {
			return bizerr.NewCode(customer.CodeUnavailable)
		}
		now, txErr := dbNow(s.database, ctx)
		if txErr != nil {
			return txErr
		}
		customerNo, txErr := randomNo("CUS_")
		if txErr != nil {
			return txErr
		}
		contactNo, txErr := randomNo("CTC_")
		if txErr != nil {
			return txErr
		}
		bindingNo, txErr := randomNo("BND_")
		if txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,customer_type_code,status_code,display_name,search_name,country_code,preferred_locale,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,'individual','active',?,?,?,?,1,?,?,?,?)`, customerNo, value.DisplayName, strings.ToLower(value.DisplayName), value.CountryCode, value.PreferredLocale, staffID, staffID, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_individual_profile (tenant_id,customer_no,customer_type_code,created_at,updated_at) VALUES (0,?,'individual',?,?)`, customerNo, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_enterprise_profile (tenant_id,customer_no,customer_type_code,created_at,updated_at) VALUES (0,?,'enterprise',?,?)`, customerNo, now, now); txErr != nil {
			return txErr
		}

		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact (tenant_id,contact_no,customer_no,display_name,email,email_lookup_digest,phone_country_code,phone,phone_lookup_digest,preferred_locale,status_code,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,'active',1,?,?,?,?)`, contactNo, customerNo, value.DisplayName, value.Email, emailDigest, value.PhoneCountryCode, value.Phone, phoneDigest, value.PreferredLocale, staffID, staffID, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact_role (tenant_id,customer_no,contact_no,role_code,status_code,assigned_by,assigned_at,created_at,updated_at) VALUES (0,?,?,'primary','active',?,?,?,?)`, customerNo, contactNo, staffID, now, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_principal_audience (tenant_id,sys_user_id,audience_code,status_code,created_by,updated_by,created_at,updated_at) VALUES (0,?,'customer','active',?,?,?,?)`, userID, staffID, staffID, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_identity_binding (tenant_id,binding_no,sys_user_id,customer_no,source_code,source_reference_no,status_code,activated_at,must_change_password,idempotency_key_digest,request_fingerprint,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,'admin_direct',?,'active',?,FALSE,?,?,1,?,?,?,?)`, bindingNo, userID, customerNo, bindingNo, now, requestDigest, requestFingerprint, staffID, staffID, now, now); txErr != nil {
			return txErr
		}
		if txErr = s.loginRecorder.RecordProvisioned(ctx, userID, loginName, value.Email, loginPhone); txErr != nil {
			return txErr
		}
		if txErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "customer.direct.create", IdempotencyKey: bindingNo, OperationCode: "customer.direct.create", TargetType: "customer", TargetNo: customerNo, CustomerNo: customerNo, ResultCode: "succeeded", ReasonCode: "admin_direct", RiskCode: "medium", RequiredPermission: PermissionCustomerCreate, CorrelationNo: bindingNo, BeforeSnapshot: map[string]any{}, AfterSnapshot: map[string]any{"customer_type": value.CustomerType, "status": "active", "login_kind": value.contactKind, "must_change_password": false}}); txErr != nil {
			return txErr
		}
		result = &DirectAccount{CustomerNo: customerNo, UserID: userID, Status: "active"}
		return nil
	})
	password = ""
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

// ResetPassword replaces an active customer's host password without a first-login gate.
// The host owns the password hash; business audit never receives the plaintext.
func (s *serviceImpl) ResetPassword(ctx context.Context, input PasswordResetInput) error {
	staffID, err := s.authorizeStaff(ctx, PermissionCustomerUpdate)
	if err != nil {
		return err
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	password := strings.TrimSpace(input.NewPassword)
	input.NewPassword = ""
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !activationCustomerNoPattern.MatchString(input.CustomerNo) || utf8.RuneCountInString(password) < 8 || utf8.RuneCountInString(password) > 128 || utf8.RuneCountInString(input.Reason) < 10 || utf8.RuneCountInString(input.Reason) > 512 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return bizerr.NewCode(customer.CodeProfileInvalid)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row struct {
			UserID             int  `orm:"sys_user_id"`
			MustChangePassword bool `orm:"must_change_password"`
		}
		if txErr := s.database.GetScan(ctx, &row, `SELECT binding.sys_user_id,binding.must_change_password FROM plugin_idc_core_customer_identity_binding binding JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=binding.tenant_id AND customer.customer_no=binding.customer_no WHERE binding.tenant_id=0 AND binding.customer_no=? AND binding.status_code='active' AND customer.status_code='active' FOR UPDATE`, input.CustomerNo); txErr != nil {
			return txErr
		}
		if row.UserID <= 0 {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		if txErr := s.users.ResetPassword(ctx, usercap.UserID(strconv.Itoa(row.UserID)), password); txErr != nil {
			return txErr
		}
		now, txErr := dbNow(s.database, ctx)
		if txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_customer_identity_binding SET must_change_password=FALSE,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND sys_user_id=? AND status_code='active'`, staffID, now, input.CustomerNo, row.UserID); txErr != nil {
			return txErr
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "customer.password.reset", IdempotencyKey: input.IdempotencyKey, OperationCode: "customer.password.reset", TargetType: "customer", TargetNo: input.CustomerNo, CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: "staff_reset", RiskCode: "high", RequiredPermission: PermissionCustomerUpdate, CorrelationNo: input.IdempotencyKey, BeforeSnapshot: map[string]any{"must_change_password": row.MustChangePassword}, AfterSnapshot: map[string]any{"must_change_password": false, "reason": input.Reason}})
	})
	password = ""
	if err != nil {
		return wrap(err)
	}
	return nil
}

type ExpiryWorker interface {
	Expire(context.Context, int) (int, error)
}

// LoginRecorder receives only successfully provisioned host identity values.
type LoginRecorder interface {
	// DefaultRoleID resolves the enabled public-registration self-only role.
	DefaultRoleID(context.Context) (int, error)
	// RecordProvisioned joins the account owner's transaction and publishes only
	// the exact username and contacts accepted by user creation.
	RecordProvisioned(context.Context, int, string, string, string) error
}

type serviceImpl struct {
	loginRecorder LoginRecorder
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	users         usercap.Service
	audit         idccap.BusinessAuditWriter
	notifications notificationcap.EventSink
	lookupKey     []byte
	publicURL     func(string) (string, error)
}

func New(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, users usercap.Service, loginRecorder LoginRecorder, auditWriter idccap.BusinessAuditWriter, notifications notificationcap.EventSink, lookupKey []byte, publicURL func(string) (string, error)) (Service, error) {
	if database == nil || bizCtx == nil || principalReader == nil || authorization == nil || users == nil || loginRecorder == nil || auditWriter == nil || notifications == nil || len(lookupKey) != sha256.Size || publicURL == nil {
		return nil, gerror.New("customer activation requires all governed dependencies")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, principal: principalReader, authorization: authorization, users: users, loginRecorder: loginRecorder, audit: auditWriter, notifications: notifications, lookupKey: append([]byte(nil), lookupKey...), publicURL: publicURL}, nil
}

type expiryWorker struct{ database gdb.DB }

func NewExpiryWorker(database gdb.DB) (ExpiryWorker, error) {
	if database == nil {
		return nil, gerror.New("customer activation expiry requires database")
	}
	return &expiryWorker{database: database}, nil
}

func (w *expiryWorker) Expire(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > 1000 {
		return 0, gerror.New("customer activation expiry input is invalid")
	}
	result, err := w.database.Exec(ctx, `WITH due AS (
SELECT id FROM plugin_idc_core_customer_activation_invitation
WHERE tenant_id=0 AND status_code='active' AND expires_at<=CURRENT_TIMESTAMP
ORDER BY expires_at,id LIMIT ? FOR UPDATE SKIP LOCKED
)
UPDATE plugin_idc_core_customer_activation_invitation invitation
SET status_code='expired',updated_at=CURRENT_TIMESTAMP
FROM due WHERE invitation.id=due.id AND invitation.status_code='active'`, limit)
	if err != nil {
		return 0, gerror.Wrap(err, "expire customer activation invitations")
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

type normalizedCreate struct {
	CreateInput
	contactKind  string
	contactValue string
	fingerprint  string
}

func (s *serviceImpl) Create(ctx context.Context, input CreateInput) (*Invitation, error) {
	staffID, err := s.authorizeStaff(ctx, PermissionCustomerCreate)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, input, staffID, "admin_invitation", PermissionCustomerCreate)
}

// CreateForChannel creates the same 24-hour activation invitation as the
// administrator flow, but fixes provenance to the trusted source-plugin
// runtime and never accepts a caller-selected existing host identity.
func (s *serviceImpl) CreateForChannel(
	ctx context.Context,
	channelID string,
	input CreateInput,
) (*Invitation, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" || len(channelID) > 80 || input.BindExistingIdentity || input.TargetUserID != 0 {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	input.BindExistingIdentity = false
	input.TargetUserID = 0
	input.Reason = "Delegated customer created by channel " + channelID
	input.IdempotencyKey = "channel:" + digest(channelID+"\x00"+strings.TrimSpace(input.IdempotencyKey))
	return s.create(ctx, input, 0, "channel_delegation", "")
}

func (s *serviceImpl) create(
	ctx context.Context,
	input CreateInput,
	createdBy int,
	reasonCode string,
	requiredPermission authz.PermissionKey,
) (*Invitation, error) {
	value, err := normalizeCreate(input)
	if err != nil {
		return nil, err
	}
	if value.BindExistingIdentity {
		value.TargetUserID, err = s.resolveTargetIdentity(ctx, value.contactValue)
		if err != nil {
			return nil, err
		}
		value.fingerprint = createFingerprint(value)
	}
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return nil, wrap(err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	var result *Invitation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, txErr := dbNow(s.database, ctx)
		if txErr != nil {
			return txErr
		}
		var replay *invitationRow
		txErr = s.database.GetScan(ctx, &replay, `SELECT i.invitation_no,i.customer_no,i.target_user_id,i.status_code,i.expires_at,i.request_fingerprint,c.customer_type_code,c.display_name,ct.email,ct.phone_country_code,ct.phone FROM plugin_idc_core_customer_activation_invitation i JOIN plugin_idc_core_customer_subject c ON c.customer_no=i.customer_no JOIN plugin_idc_core_customer_contact ct ON ct.customer_no=i.customer_no AND ct.contact_no=i.contact_no WHERE i.tenant_id=0 AND i.created_by=? AND i.idempotency_key_digest=? FOR UPDATE`, createdBy, digest(value.IdempotencyKey))
		if txErr != nil {
			return txErr
		}
		if replay != nil {
			if replay.RequestFingerprint != value.fingerprint {
				return bizerr.NewCode(customer.CodeIdempotencyConflict)
			}
			result = replay.project(true)
			return nil
		}

		destinationDigest := s.destinationDigest(value.contactKind, value.contactValue)
		column := "email_lookup_digest"
		if value.contactKind == "sms" {
			column = "phone_lookup_digest"
		}
		duplicate, duplicateErr := s.database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_customer_contact WHERE tenant_id=0 AND status_code='active' AND `+column+`=?`, destinationDigest)
		if duplicateErr != nil || duplicate.Int() != 0 {
			if duplicateErr != nil {
				return duplicateErr
			}
			return duplicateContactError(value.contactKind)
		}
		if value.TargetUserID > 0 {
			if txErr = s.ensureUserUnbound(ctx, value.TargetUserID); txErr != nil {
				return txErr
			}
		}

		customerNo, txErr := randomNo("CUS_")
		if txErr != nil {
			return txErr
		}
		contactNo, txErr := randomNo("CTC_")
		if txErr != nil {
			return txErr
		}
		invitationNo, txErr := randomNo("INV_")
		if txErr != nil {
			return txErr
		}
		expiresAt := now.Add(invitationDuration)
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,customer_type_code,status_code,display_name,search_name,country_code,preferred_locale,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,'individual','pending_activation',?,?,?,?,1,?,?,?,?)`, customerNo, value.DisplayName, strings.ToLower(value.DisplayName), value.CountryCode, value.PreferredLocale, createdBy, createdBy, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_individual_profile (tenant_id,customer_no,customer_type_code,created_at,updated_at) VALUES (0,?,'individual',?,?)`, customerNo, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_enterprise_profile (tenant_id,customer_no,customer_type_code,created_at,updated_at) VALUES (0,?,'enterprise',?,?)`, customerNo, now, now); txErr != nil {
			return txErr
		}
		email, emailDigest, callingCode, phone, phoneDigest := "", "", "", "", ""
		if value.contactKind == "email" {
			email, emailDigest = value.Email, destinationDigest
		} else {
			callingCode, phone, phoneDigest = value.PhoneCountryCode, value.Phone, destinationDigest
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact (tenant_id,contact_no,customer_no,display_name,email,email_lookup_digest,phone_country_code,phone,phone_lookup_digest,preferred_locale,status_code,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,'active',1,?,?,?,?)`, contactNo, customerNo, value.DisplayName, email, emailDigest, callingCode, phone, phoneDigest, value.PreferredLocale, createdBy, createdBy, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact_role (tenant_id,customer_no,contact_no,role_code,status_code,assigned_by,assigned_at,created_at,updated_at) VALUES (0,?,?,'primary','active',?,?,?,?)`, customerNo, contactNo, createdBy, now, now, now); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_activation_invitation (tenant_id,invitation_no,customer_no,contact_no,token_digest,target_user_id,reason,idempotency_key_digest,request_fingerprint,status_code,expires_at,created_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?, 'active',?,?,?,?)`, invitationNo, customerNo, contactNo, digest(token), nullableUser(value.TargetUserID), value.Reason, digest(value.IdempotencyKey), value.fingerprint, expiresAt, createdBy, now, now); txErr != nil {
			return txErr
		}
		launchURL, txErr := s.publicURL("/auth/customer-activation?token=" + token)
		if txErr != nil {
			return txErr
		}
		if _, txErr = s.notifications.Enqueue(ctx, notificationcap.EventRequest{EventCode: "customer.activation_invitation", Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact, CustomerNo: customerNo, ContactNo: contactNo}, BusinessObjectType: notificationcap.BusinessObjectAccount, BusinessObjectNo: customerNo, BusinessObjectVersion: 1, Variables: map[string]string{"activation_url": launchURL, "customer_display_name": value.DisplayName, "expires_at": expiresAt.Format(time.RFC3339)}, CorrelationNo: invitationNo, IdempotencyKey: invitationNo}); txErr != nil {
			return txErr
		}
		if txErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "customer.activation.create", IdempotencyKey: invitationNo, OperationCode: "customer.activation.create", TargetType: "customer", TargetNo: customerNo, CustomerNo: customerNo, ResultCode: "succeeded", ReasonCode: reasonCode, RiskCode: "medium", RequiredPermission: string(requiredPermission), CorrelationNo: invitationNo, BeforeSnapshot: map[string]any{}, AfterSnapshot: map[string]any{"customer_type": value.CustomerType, "status": "pending_activation", "target_identity_selected": value.TargetUserID > 0}}); txErr != nil {
			return txErr
		}
		result = (&invitationRow{InvitationNo: invitationNo, CustomerNo: customerNo, CustomerType: value.CustomerType, DisplayName: value.DisplayName, Email: email, PhoneCountryCode: callingCode, Phone: phone, TargetUserID: int64(value.TargetUserID), Status: "active", ExpiresAt: expiresAt}).project(false)
		return nil
	})
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *serviceImpl) Resend(ctx context.Context, input ResendInput) (*Invitation, error) {
	staffID, err := s.authorizeStaff(ctx, PermissionCustomerCreate)
	if err != nil {
		return nil, err
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !activationCustomerNoPattern.MatchString(input.CustomerNo) ||
		utf8.RuneCountInString(input.Reason) < 10 || utf8.RuneCountInString(input.Reason) > 512 ||
		len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	fingerprint := digest(strings.Join([]string{"resend", input.CustomerNo, input.Reason}, "\x00"))
	var result *Invitation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, txErr := dbNow(s.database, ctx)
		if txErr != nil {
			return txErr
		}
		var replay *invitationRow
		if txErr = s.database.GetScan(ctx, &replay, `SELECT i.invitation_no,i.customer_no,i.target_user_id,i.status_code,i.expires_at,i.request_fingerprint,c.customer_type_code,c.display_name,ct.email,ct.phone_country_code,ct.phone FROM plugin_idc_core_customer_activation_invitation i JOIN plugin_idc_core_customer_subject c ON c.customer_no=i.customer_no JOIN plugin_idc_core_customer_contact ct ON ct.customer_no=i.customer_no AND ct.contact_no=i.contact_no WHERE i.tenant_id=0 AND i.created_by=? AND i.idempotency_key_digest=? FOR UPDATE`, staffID, digest(input.IdempotencyKey)); txErr != nil {
			return txErr
		}
		if replay != nil {
			if replay.RequestFingerprint != fingerprint || replay.CustomerNo != input.CustomerNo {
				return bizerr.NewCode(customer.CodeIdempotencyConflict)
			}
			result = replay.project(true)
			return nil
		}

		var current *resendRow
		if txErr = s.database.GetScan(ctx, &current, `SELECT i.id,i.invitation_no,i.customer_no,i.contact_no,i.target_user_id,i.status_code,i.expires_at,c.customer_type_code,c.display_name,c.status_code AS customer_status,ct.email,ct.phone_country_code,ct.phone FROM plugin_idc_core_customer_subject c JOIN plugin_idc_core_customer_activation_invitation i ON i.customer_no=c.customer_no JOIN plugin_idc_core_customer_contact ct ON ct.customer_no=i.customer_no AND ct.contact_no=i.contact_no WHERE c.tenant_id=0 AND c.customer_no=? AND i.status_code IN ('active','expired') ORDER BY i.id DESC LIMIT 1 FOR UPDATE OF c,i`, input.CustomerNo); txErr != nil {
			return txErr
		}
		if current == nil || current.CustomerStatus != "pending_activation" {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		if current.TargetUserID != nil {
			if txErr = s.ensureUserUnbound(ctx, int(*current.TargetUserID)); txErr != nil {
				return txErr
			}
		}
		if _, txErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_customer_activation_invitation SET status_code=CASE WHEN expires_at<=? THEN 'expired' ELSE 'superseded' END,superseded_at=CASE WHEN expires_at>? THEN CURRENT_TIMESTAMP ELSE NULL END,updated_at=? WHERE tenant_id=0 AND customer_no=? AND status_code='active'`, now, now, now, input.CustomerNo); txErr != nil {
			return txErr
		}
		tokenBytes := make([]byte, 32)
		if _, txErr = rand.Read(tokenBytes); txErr != nil {
			return txErr
		}
		token := base64.RawURLEncoding.EncodeToString(tokenBytes)
		invitationNo, txErr := randomNo("INV_")
		if txErr != nil {
			return txErr
		}
		expiresAt := now.Add(invitationDuration)
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_activation_invitation (tenant_id,invitation_no,customer_no,contact_no,token_digest,target_user_id,reason,idempotency_key_digest,request_fingerprint,status_code,expires_at,created_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?, 'active',?,?,?,?)`, invitationNo, input.CustomerNo, current.ContactNo, digest(token), nullableInt64(current.TargetUserID), input.Reason, digest(input.IdempotencyKey), fingerprint, expiresAt, staffID, now, now); txErr != nil {
			return txErr
		}
		launchURL, txErr := s.publicURL("/auth/customer-activation?token=" + token)
		if txErr != nil {
			return txErr
		}
		if _, txErr = s.notifications.Enqueue(ctx, notificationcap.EventRequest{EventCode: "customer.activation_invitation", Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact, CustomerNo: input.CustomerNo, ContactNo: current.ContactNo}, BusinessObjectType: notificationcap.BusinessObjectAccount, BusinessObjectNo: input.CustomerNo, BusinessObjectVersion: 1, Variables: map[string]string{"activation_url": launchURL, "customer_display_name": current.DisplayName, "expires_at": expiresAt.Format(time.RFC3339)}, CorrelationNo: invitationNo, IdempotencyKey: invitationNo}); txErr != nil {
			return txErr
		}
		if txErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "customer.activation.resend", IdempotencyKey: invitationNo, OperationCode: "customer.activation.resend", TargetType: "customer", TargetNo: input.CustomerNo, CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: "admin_resend", RiskCode: "medium", RequiredPermission: PermissionCustomerCreate, CorrelationNo: invitationNo, BeforeSnapshot: map[string]any{"invitation_no": current.InvitationNo, "status": current.Status}, AfterSnapshot: map[string]any{"invitation_no": invitationNo, "status": "active", "expires_at": expiresAt}}); txErr != nil {
			return txErr
		}
		targetUserID := int64(0)
		if current.TargetUserID != nil {
			targetUserID = *current.TargetUserID
		}
		result = (&invitationRow{InvitationNo: invitationNo, CustomerNo: input.CustomerNo, CustomerType: current.CustomerType, DisplayName: current.DisplayName, Email: current.Email, PhoneCountryCode: current.PhoneCountryCode, Phone: current.Phone, TargetUserID: targetUserID, Status: "active", ExpiresAt: expiresAt, RequestFingerprint: fingerprint}).project(false)
		return nil
	})
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *serviceImpl) Redeem(ctx context.Context, rawToken string) (*Activation, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	if err := s.ensureCustomerEligible(ctx); err != nil {
		return nil, err
	}
	rawToken = strings.TrimSpace(rawToken)
	if len(rawToken) < 32 || len(rawToken) > 128 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	var result *Activation
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, txErr := dbNow(s.database, ctx)
		if txErr != nil {
			return txErr
		}
		var invitation *redeemRow
		if txErr = s.database.GetScan(ctx, &invitation, `SELECT i.id,i.invitation_no,i.customer_no,i.contact_no,i.target_user_id,i.status_code,i.expires_at,i.consumed_by_user_id,c.customer_type_code,c.status_code AS customer_status FROM plugin_idc_core_customer_activation_invitation i JOIN plugin_idc_core_customer_subject c ON c.customer_no=i.customer_no WHERE i.tenant_id=0 AND i.token_digest=? FOR UPDATE OF i,c`, digest(rawToken)); txErr != nil {
			return txErr
		}
		if invitation == nil {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		if invitation.Status == "consumed" && invitation.ConsumedByUserID == int64(current.UserID) && invitation.CustomerStatus == "active" {
			result = &Activation{CustomerNo: invitation.CustomerNo, CustomerType: invitation.CustomerType, Status: "active", Reused: true}
			return nil
		}
		if invitation.Status != "active" || !now.Before(invitation.ExpiresAt) || invitation.CustomerStatus != "pending_activation" || (invitation.TargetUserID != nil && *invitation.TargetUserID != int64(current.UserID)) {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		if txErr = s.ensureUserUnbound(ctx, current.UserID); txErr != nil {
			return txErr
		}
		bindingNo, txErr := randomNo("BND_")
		if txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_principal_audience (tenant_id,sys_user_id,audience_code,status_code,created_by,updated_by,created_at,updated_at) VALUES (0,?,'customer','active',?,?,?,?)`, current.UserID, current.UserID, current.UserID, now, now); txErr != nil {
			return bizerr.WrapCode(txErr, customer.CodeNotAccessible)
		}
		if _, txErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_identity_binding (tenant_id,binding_no,sys_user_id,customer_no,source_code,source_reference_no,status_code,activated_at,version,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,'admin_invitation',?,'active',?,1,?,?,?,?)`, bindingNo, current.UserID, invitation.CustomerNo, invitation.InvitationNo, now, current.UserID, current.UserID, now, now); txErr != nil {
			return bizerr.WrapCode(txErr, customer.CodeNotAccessible)
		}
		if _, txErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_customer_subject SET status_code='active',version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND status_code='pending_activation'`, current.UserID, now, invitation.CustomerNo); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_customer_contact SET email_verified_at=CASE WHEN email<>'' THEN ? ELSE email_verified_at END,phone_verified_at=CASE WHEN phone<>'' THEN ? ELSE phone_verified_at END,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND contact_no=?`, now, now, current.UserID, now, invitation.CustomerNo, invitation.ContactNo); txErr != nil {
			return txErr
		}
		if _, txErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_customer_activation_invitation SET status_code='consumed',consumed_by_user_id=?,consumed_at=?,updated_at=? WHERE id=? AND status_code='active'`, current.UserID, now, now, invitation.ID); txErr != nil {
			return txErr
		}
		if txErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "customer.activation.redeem", IdempotencyKey: invitation.InvitationNo, OperationCode: "customer.activation.redeem", TargetType: "customer", TargetNo: invitation.CustomerNo, CustomerNo: invitation.CustomerNo, ResultCode: "succeeded", ReasonCode: "admin_invitation", RiskCode: "medium", CorrelationNo: invitation.InvitationNo, BeforeSnapshot: map[string]any{"status": "pending_activation"}, AfterSnapshot: map[string]any{"status": "active", "binding_source": "admin_invitation"}}); txErr != nil {
			return txErr
		}
		result = &Activation{CustomerNo: invitation.CustomerNo, CustomerType: invitation.CustomerType, Status: "active"}
		return nil
	})
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

type invitationRow struct {
	InvitationNo       string    `orm:"invitation_no"`
	CustomerNo         string    `orm:"customer_no"`
	CustomerType       string    `orm:"customer_type_code"`
	DisplayName        string    `orm:"display_name"`
	Email              string    `orm:"email"`
	PhoneCountryCode   string    `orm:"phone_country_code"`
	Phone              string    `orm:"phone"`
	TargetUserID       int64     `orm:"target_user_id"`
	Status             string    `orm:"status_code"`
	ExpiresAt          time.Time `orm:"expires_at"`
	RequestFingerprint string    `orm:"request_fingerprint"`
}

func (r *invitationRow) project(reused bool) *Invitation {
	hint := maskDestination(r.Email, r.PhoneCountryCode, r.Phone)
	return &Invitation{InvitationNo: r.InvitationNo, CustomerNo: r.CustomerNo, CustomerType: r.CustomerType, DisplayName: r.DisplayName, DestinationHint: hint, TargetUserID: int(r.TargetUserID), ExistingIdentityBound: r.TargetUserID > 0, Status: r.Status, ExpiresAt: r.ExpiresAt, Reused: reused}
}

type redeemRow struct {
	ID               int64     `orm:"id"`
	InvitationNo     string    `orm:"invitation_no"`
	CustomerNo       string    `orm:"customer_no"`
	ContactNo        string    `orm:"contact_no"`
	TargetUserID     *int64    `orm:"target_user_id"`
	Status           string    `orm:"status_code"`
	ExpiresAt        time.Time `orm:"expires_at"`
	ConsumedByUserID int64     `orm:"consumed_by_user_id"`
	CustomerType     string    `orm:"customer_type_code"`
	CustomerStatus   string    `orm:"customer_status"`
}

type resendRow struct {
	ID               int64     `orm:"id"`
	InvitationNo     string    `orm:"invitation_no"`
	CustomerNo       string    `orm:"customer_no"`
	ContactNo        string    `orm:"contact_no"`
	TargetUserID     *int64    `orm:"target_user_id"`
	Status           string    `orm:"status_code"`
	ExpiresAt        time.Time `orm:"expires_at"`
	CustomerType     string    `orm:"customer_type_code"`
	DisplayName      string    `orm:"display_name"`
	CustomerStatus   string    `orm:"customer_status"`
	Email            string    `orm:"email"`
	PhoneCountryCode string    `orm:"phone_country_code"`
	Phone            string    `orm:"phone"`
}

func (s *serviceImpl) authorizeStaff(ctx context.Context, permission authz.PermissionKey) (int, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	keys := []authz.PermissionKey{permission, idccap.HostPermissionWildcard}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err == nil && (grants[keys[0]] || grants[keys[1]]) {
		return current.UserID, nil
	}
	admin, adminErr := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || adminErr != nil || !admin {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return current.UserID, nil
}

func duplicateContactError(kind string) error {
	if kind == "sms" {
		return bizerr.NewCode(customer.CodePhoneExists)
	}
	return bizerr.NewCode(customer.CodeEmailExists)
}

func (s *serviceImpl) ensureCustomerEligible(ctx context.Context) error {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience != nil {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	keys := make([]authz.PermissionKey, 0, len(idccap.AdminPermissionCatalog())+1)
	keys = append(keys, idccap.HostPermissionWildcard)
	for _, item := range idccap.AdminPermissionCatalog() {
		keys = append(keys, authz.PermissionKey(item))
	}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return bizerr.WrapCode(err, customer.CodeNotAccessible)
	}
	for _, granted := range grants {
		if granted {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
	}
	current := s.bizCtx.Current(ctx)
	admin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || admin {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	return nil
}

func (s *serviceImpl) resolveTargetIdentity(ctx context.Context, contact string) (int, error) {
	result, err := s.users.BatchResolve(ctx, usercap.BatchResolveInput{Contacts: []string{contact}})
	if err != nil || result == nil {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	resolved := 0
	for _, item := range result.Items {
		id, parseErr := strconv.Atoi(string(item.ID))
		if parseErr != nil || id <= 0 || (resolved != 0 && resolved != id) {
			return 0, bizerr.NewCode(customer.CodeNotAccessible)
		}
		resolved = id
	}
	if resolved == 0 {
		return 0, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return resolved, nil
}

func (s *serviceImpl) ensureUserUnbound(ctx context.Context, userID int) error {
	count, err := s.database.GetValue(ctx, `SELECT (SELECT COUNT(1) FROM plugin_idc_core_principal_audience WHERE tenant_id=0 AND sys_user_id=?) + (SELECT COUNT(1) FROM plugin_idc_core_customer_identity_binding WHERE tenant_id=0 AND sys_user_id=?)`, userID, userID)
	if err != nil {
		return err
	}
	if count.Int() != 0 {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	return nil
}

func normalizeCreate(input CreateInput) (normalizedCreate, error) {
	input.CustomerType = strings.TrimSpace(input.CustomerType)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.CountryCode = strings.ToUpper(strings.TrimSpace(input.CountryCode))
	input.PreferredLocale = strings.TrimSpace(input.PreferredLocale)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.PhoneCountryCode = strings.TrimSpace(input.PhoneCountryCode)
	input.Phone = strings.ReplaceAll(strings.TrimSpace(input.Phone), " ", "")
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.CustomerType != "individual" || !utf8.ValidString(input.DisplayName) || len(input.DisplayName) < 1 || len(input.DisplayName) > 256 || !activationCountryPattern.MatchString(input.CountryCode) || (input.PreferredLocale != "zh-CN" && input.PreferredLocale != "en-US") || !utf8.ValidString(input.Reason) || utf8.RuneCountInString(input.Reason) < 10 || utf8.RuneCountInString(input.Reason) > 512 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 || input.TargetUserID < 0 {
		return normalizedCreate{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	value := normalizedCreate{CreateInput: input}
	if input.Email != "" && input.Phone == "" && input.PhoneCountryCode == "" {
		parsed, err := mail.ParseAddress(input.Email)
		if err != nil || parsed.Address != input.Email || len(input.Email) > 320 {
			return normalizedCreate{}, bizerr.NewCode(customer.CodeProfileInvalid)
		}
		value.contactKind, value.contactValue = "email", input.Email
	} else if input.Email == "" && activationCallingPattern.MatchString(input.PhoneCountryCode) && activationPhonePattern.MatchString(input.Phone) {
		value.contactKind, value.contactValue = "sms", input.PhoneCountryCode+input.Phone
	} else {
		return normalizedCreate{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	value.fingerprint = createFingerprint(value)
	return value, nil
}

func createFingerprint(value normalizedCreate) string {
	return digest(strings.Join([]string{value.CustomerType, value.DisplayName, value.CountryCode, value.PreferredLocale, value.contactKind, value.contactValue, strconv.FormatBool(value.BindExistingIdentity), strconv.Itoa(value.TargetUserID), value.Reason}, "\x00"))
}

// directLoginName creates a stable independent name; contacts remain login aliases.
func directLoginName(value normalizedCreate) string {
	return "customer_" + digest(value.contactKind + "\x00" + value.contactValue)[:24]
}

func (s *serviceImpl) destinationDigest(kind, value string) string {
	mac := hmac.New(sha256.New, s.lookupKey)
	_, _ = mac.Write([]byte("customer-contact-" + kind + "-v1\x00" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

func dbNow(database gdb.DB, ctx context.Context) (time.Time, error) {
	value, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, err
	}
	return value.Time(), nil
}

func randomNo(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func nullableUser(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
func maskDestination(email, code, phone string) string {
	if email != "" {
		parts := strings.SplitN(email, "@", 2)
		if len(parts) == 2 {
			return parts[0][:1] + "***@" + parts[1]
		}
	}
	if len(phone) >= 4 {
		return code + " ***" + phone[len(phone)-4:]
	}
	return "***"
}
func wrap(err error) error {
	if bizerr.Is(err, customer.CodeNotAccessible) || bizerr.Is(err, customer.CodeProfileInvalid) || bizerr.Is(err, customer.CodePhoneExists) || bizerr.Is(err, customer.CodeEmailExists) || bizerr.Is(err, customer.CodeIdempotencyConflict) {
		return err
	}
	return bizerr.WrapCode(err, customer.CodeUnavailable)
}
