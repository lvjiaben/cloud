// This file appends the separate sensitive-information consent owned by core.

package identityprojection

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	personalConsentNumberType     = "customer_consent_sensitive_personal_information"
	personalConsentNumberOwner    = "customer_consent"
	personalConsentNumberPrefix   = "CNS_"
	personalConsentSource         = "portal"
	personalConsentNumberAttempts = 16
)

// RecordPersonalConsent appends or safely replays one current-customer grant.
func (s *serviceImpl) RecordPersonalConsent(
	ctx context.Context,
	input identitycap.PersonalConsentInput,
) (*identitycap.PersonalConsent, error) {
	if s == nil || ctx == nil || ctx.Err() != nil ||
		input.PolicyVersion != identitycap.PersonalConsentPolicyVersion ||
		!validConsentLocale(input.Locale) || !validConsentIdempotencyKey(input.IdempotencyKey) {
		return nil, gerror.New("personal identity consent input is invalid")
	}
	subject, err := s.CurrentPersonal(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	var result *identitycap.PersonalConsent
	err = dao.CustomerConsent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		consentNo, reserveErr := s.reservePersonalConsentNumber(
			ctx, subject.CustomerNo, input.IdempotencyKey,
		)
		if reserveErr != nil {
			return reserveErr
		}
		var existing *entity.CustomerConsent
		if loadErr := dao.CustomerConsent.Ctx(ctx).Where(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, ConsentNo: consentNo,
			CustomerNo: subject.CustomerNo,
		}).Limit(1).Scan(&existing); loadErr != nil {
			return gerror.Wrap(loadErr, "load personal identity consent")
		}
		if existing != nil {
			if !matchesPersonalConsent(existing, input, subject.ActorUserID) {
				return gerror.New("personal identity consent idempotency conflict")
			}
			result = &identitycap.PersonalConsent{
				ConsentNo: existing.ConsentNo, PolicyVersion: existing.PolicyVersion,
				OccurredAt: existing.OccurredAt.UTC(), Reused: true,
			}
			return nil
		}
		evidence := s.personalConsentDigest(
			"personal-consent-evidence-v1",
			subject.CustomerNo+"\x00"+consentNo+"\x00"+
				identitycap.PersonalConsentPolicyReference+"\x00"+input.PolicyVersion+"\x00"+
				input.Locale+"\x00"+strconv.Itoa(subject.ActorUserID),
		)
		rowID, insertErr := dao.CustomerConsent.Ctx(ctx).Data(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, ConsentNo: consentNo,
			CustomerNo: subject.CustomerNo, ConsentTypeCode: sensitiveConsentType,
			PolicyReference: identitycap.PersonalConsentPolicyReference,
			PolicyVersion:   input.PolicyVersion, ActionCode: consentGranted,
			SourceCode: personalConsentSource, Locale: input.Locale,
			EvidenceDigest: evidence, OccurredAt: &now, RecordedBy: subject.ActorUserID,
		}).InsertAndGetId()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert personal identity consent")
		}
		if assignErr := assignPersonalConsentNumber(ctx, consentNo, rowID, now); assignErr != nil {
			return assignErr
		}
		result = &identitycap.PersonalConsent{
			ConsentNo: consentNo, PolicyVersion: input.PolicyVersion, OccurredAt: now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serviceImpl) reservePersonalConsentNumber(
	ctx context.Context,
	customerNo string,
	idempotencyKey string,
) (string, error) {
	request := do.BusinessNumber{
		TenantId: idccap.DefaultTenantID, NumberType: personalConsentNumberType,
		ReservationScopeDigest: s.personalConsentDigest("personal-consent-scope-v1", customerNo),
		IdempotencyKeyDigest:   s.personalConsentDigest("personal-consent-idempotency-v1", idempotencyKey),
	}
	if existing, err := loadPersonalConsentNumber(ctx, request); err != nil || existing != "" {
		return existing, err
	}
	for range personalConsentNumberAttempts {
		candidate, err := newPersonalConsentNumber(s.random)
		if err != nil {
			return "", err
		}
		inserted, err := dao.BusinessNumber.Ctx(ctx).Data(do.BusinessNumber{
			TenantId: idccap.DefaultTenantID, NumberType: personalConsentNumberType,
			BusinessNo: candidate, ReservationScopeDigest: request.ReservationScopeDigest,
			IdempotencyKeyDigest: request.IdempotencyKeyDigest,
		}).InsertIgnore()
		if err != nil {
			return "", gerror.Wrap(err, "reserve personal identity consent number")
		}
		affected, err := inserted.RowsAffected()
		if err != nil {
			return "", gerror.Wrap(err, "read personal identity consent number reservation")
		}
		if affected == 1 {
			return candidate, nil
		}
		if existing, loadErr := loadPersonalConsentNumber(ctx, request); loadErr != nil || existing != "" {
			return existing, loadErr
		}
	}
	return "", gerror.New("personal identity consent number collision retry limit reached")
}

func loadPersonalConsentNumber(ctx context.Context, request do.BusinessNumber) (string, error) {
	var row *entity.BusinessNumber
	if err := dao.BusinessNumber.Ctx(ctx).Where(request).Limit(1).Scan(&row); err != nil {
		return "", gerror.Wrap(err, "load personal identity consent number")
	}
	if row == nil {
		return "", nil
	}
	return row.BusinessNo, nil
}

func assignPersonalConsentNumber(
	ctx context.Context,
	consentNo string,
	ownerID int64,
	now time.Time,
) error {
	updated, err := dao.BusinessNumber.Ctx(ctx).
		Where(do.BusinessNumber{TenantId: idccap.DefaultTenantID, BusinessNo: consentNo}).
		Data(do.BusinessNumber{
			OwnerType: personalConsentNumberOwner, OwnerInternalId: ownerID, AssignedAt: &now,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "assign personal identity consent number")
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read personal identity consent number assignment")
	}
	if affected != 1 {
		return gerror.New("personal identity consent number reservation disappeared before assignment")
	}
	return nil
}

func newPersonalConsentNumber(random io.Reader) (string, error) {
	if random == nil {
		return "", gerror.New("personal identity consent random source is unavailable")
	}
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate personal identity consent number")
	}
	return personalConsentNumberPrefix + hex.EncodeToString(content[:]), nil
}

func (s *serviceImpl) personalConsentDigest(purpose string, value string) string {
	mac := hmac.New(sha256.New, s.lookupKey)
	_, _ = mac.Write([]byte("idc/" + purpose + "\x00"))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func matchesPersonalConsent(
	row *entity.CustomerConsent,
	input identitycap.PersonalConsentInput,
	actorUserID int,
) bool {
	return row != nil && row.OccurredAt != nil && row.ConsentTypeCode == sensitiveConsentType &&
		row.PolicyReference == identitycap.PersonalConsentPolicyReference &&
		row.PolicyVersion == input.PolicyVersion && row.ActionCode == consentGranted &&
		row.SourceCode == personalConsentSource && row.Locale == input.Locale &&
		row.RecordedBy == actorUserID
}

func validConsentLocale(value string) bool {
	return value == "zh-CN" || value == "en-US"
}

func validConsentIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 128 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
