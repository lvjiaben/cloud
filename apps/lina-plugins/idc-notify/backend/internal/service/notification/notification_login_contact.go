// This file defines the authenticated login-contact verification workflow.
// Proofs are scoped to one user, session, target field and authoritative snapshot.
package notification

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-idc-notify/backend/internal/model/entity"
	"lina-plugin-idc-notify/backend/internal/service/loginprofile"
	"strconv"
	"strings"
)

// ContactKind identifies one host login-contact field, never a free-form column.
type ContactKind string

const (
	ContactEmail             ContactKind = "email"
	ContactPhone             ContactKind = "phone"
	contactOldPurpose                    = "account.contact_old"
	contactNewPurpose                    = "account.contact_new"
	contactVerificationEvent             = "account.login_contact_verification_code"
	contactGuardTable                    = "plugin_idc_notify_login_contact_guard"
	contactPrepared                      = "prepared"
	contactSent                          = "sent"
	contactUnknown                       = "result_unknown"
	contactVerified                      = "verified"
	contactSuperseded                    = "superseded"
	contactLocked                        = "locked"
	contactCreate                        = "create"
)

// LoginContactInput binds one step to an authenticated session. Destination is
// accepted only for the new contact; the old destination is always read by server.
type LoginContactInput struct {
	Kind            ContactKind
	OldVerification string
	Destination     string
	Locale          string
	IdempotencyKey  string
}

// LoginContactVerifyInput adds a bounded one-time code to its verification step.
type LoginContactVerifyInput struct {
	LoginContactInput
	VerificationNo string
	Code           string
}

type loginContactBinding struct {
	userID            int
	kind              ContactKind
	destination       []byte
	channel           string
	locale            string
	purpose           string
	event             string
	ownerDigest       string
	sessionDigest     string
	snapshotDigest    string
	destinationDigest string
	shape             string
}

// lockContact executes one operation under a durable per-account row lock. The
// same transaction context is propagated into the original profile route.
func (s *serviceImpl) lockContact(ctx context.Context, userID int, apply func(context.Context, int64) error) error {
	if userID <= 0 {
		return bizerr.NewCode(CodeNotAccessible)
	}
	return s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := s.database.Model(contactGuardTable).Ctx(txCtx).Data(do.LoginContactGuard{TenantId: notificationTenantID, UserId: userID}).InsertIgnore(); err != nil {
			return err
		}
		var row entity.LoginContactGuard
		if err := s.database.Model(contactGuardTable).Ctx(txCtx).Where(do.LoginContactGuard{TenantId: notificationTenantID, UserId: userID}).LockUpdate().Scan(&row); err != nil {
			return err
		}
		if row.UserId != userID {
			return bizerr.NewCode(CodeUnavailable)
		}
		return apply(txCtx, row.Revision)
	})
}

// Serialize keeps authorized staff writes ordered with verified customer writes.
// The callback retains the original route permission checks.
func (s *serviceImpl) Serialize(ctx context.Context, userID int, write profilechangecap.Write) error {
	if write == nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	return publicError(s.lockContact(ctx, userID, func(txCtx context.Context, revision int64) error {
		if err := write(txCtx); err != nil {
			return err
		}
		result, err := s.database.Model(contactGuardTable).Ctx(txCtx).Where(do.LoginContactGuard{TenantId: notificationTenantID, UserId: userID, Revision: revision}).Data(do.LoginContactGuard{Revision: revision + 1}).Update()
		if err != nil {
			return err
		}
		return requireOne(result)
	}))
}

func (s *serviceImpl) currentContact(ctx context.Context, input LoginContactInput, revision int64) (*loginContactBinding, error) {
	if input.Kind != ContactEmail && input.Kind != ContactPhone || !validLocale(input.Locale) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TokenID == "" || current.TenantID != notificationTenantID || current.IsImpersonation || current.ActingUserID != 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if _, err := s.sources.Audience.Current(ctx); err != nil {
		return nil, err
	}
	profile, err := s.loginProfiles.Current(ctx)
	if err != nil || profile.ID != current.UserID {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	destination, channel, err := originalContact(profile, input.Kind)
	if err != nil {
		return nil, err
	}
	purpose := contactOldPurpose
	if input.OldVerification != "" {
		purpose = contactNewPurpose
		destination = []byte(input.Destination)
		channel = "email"
		if input.Kind == ContactPhone {
			channel = "sms"
		}
	}
	normalized, err := normalizeLoginContact(channel, string(destination))
	clear(destination)
	if err != nil {
		return nil, err
	}
	digests, err := s.hmacDigests(ctx, map[string][]byte{
		"contact_owner": []byte(strconv.Itoa(current.UserID)), "contact_session": []byte(current.TokenID),
		"contact_snapshot":    []byte(digestText(profile.Email, profile.Phone, strconv.FormatInt(revision, 10))),
		"contact_destination": normalized,
	})
	if err != nil {
		clear(normalized)
		return nil, err
	}
	binding := &loginContactBinding{userID: current.UserID, kind: input.Kind, destination: normalized, channel: channel, locale: input.Locale, purpose: purpose, event: contactVerificationEvent,
		ownerDigest: digests["contact_owner"], sessionDigest: digests["contact_session"], snapshotDigest: digests["contact_snapshot"], destinationDigest: digests["contact_destination"]}
	binding.shape = contactShape(binding, input.OldVerification)
	if purpose == contactNewPurpose {
		old, loadErr := s.loadVerificationByNo(ctx, input.OldVerification)
		if loadErr != nil || !validContactProof(old, binding, contactOldPurpose, contactShape(binding, ""), s.now().UTC()) {
			clear(normalized)
			return nil, errorOrCode(loadErr, CodeVerificationInvalid)
		}
		previous := profile.Email
		if input.Kind == ContactPhone {
			previous = profile.Phone
		}
		if previous != "" {
			if value, normErr := normalizeLoginContact(channel, previous); normErr == nil && string(value) == string(normalized) {
				clear(value)
				clear(normalized)
				return nil, bizerr.NewCode(CodeInvalid)
			}
		}
	}
	return binding, nil
}

func originalContact(profile loginprofile.Snapshot, kind ContactKind) ([]byte, string, error) {
	value, channel := profile.Email, "email"
	if kind == ContactPhone {
		value, channel = profile.Phone, "sms"
	}
	if strings.TrimSpace(value) == "" {
		if kind == ContactEmail {
			value, channel = profile.Phone, "sms"
		} else {
			value, channel = profile.Email, "email"
		}
	}
	if strings.TrimSpace(value) == "" {
		return nil, "", bizerr.NewCode(CodeContactManualVerification)
	}
	return []byte(value), channel, nil
}

func normalizeLoginContact(channel, value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if channel == "sms" && len(value) == 11 && strings.HasPrefix(value, "1") {
		value = "+86" + value
	}
	return normalizePresalesDestination(channel, []byte(value))
}

func contactShape(binding *loginContactBinding, parent string) string {
	// Old proof must not depend on the new value; the new proof binds both it and
	// the old proof number. Snapshot includes both contacts and account revision.
	if parent == "" {
		return digestText(contactOldPurpose, string(binding.kind), binding.ownerDigest, binding.sessionDigest, binding.snapshotDigest)
	}
	return digestText(contactNewPurpose, string(binding.kind), binding.ownerDigest, binding.sessionDigest, binding.snapshotDigest, parent, binding.destinationDigest)
}
