package identityprojection

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const consentWithdrawn = "withdrawn"

var _ identitycap.PrivacyConsentService = (*serviceImpl)(nil)

// ListCurrentPrivacyConsents returns only the current customer's latest
// personal and enterprise decisions in one bounded query.
func (s *serviceImpl) ListCurrentPrivacyConsents(
	ctx context.Context,
) ([]identitycap.PrivacyConsentDecision, error) {
	subject, err := s.CurrentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	var rows []entity.CustomerConsent
	err = s.database.Model(dao.CustomerConsent.Table()).Ctx(ctx).Fields(
		"DISTINCT ON (consent_type_code) consent_no, consent_type_code, policy_reference, policy_version, action_code, locale, occurred_at, id",
	).Where(do.CustomerConsent{
		TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo,
	}).WhereIn("consent_type_code", []string{sensitiveConsentType, enterpriseConsentType}).
		OrderAsc("consent_type_code").OrderDesc("occurred_at").OrderDesc("id").Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "list current privacy consent decisions")
	}
	result := make([]identitycap.PrivacyConsentDecision, 0, len(rows))
	for _, row := range rows {
		if row.OccurredAt == nil {
			return nil, gerror.New("current privacy consent decision is invalid")
		}
		result = append(result, identitycap.PrivacyConsentDecision{
			ConsentNo: row.ConsentNo, ConsentType: identitycap.PrivacyConsentType(row.ConsentTypeCode),
			PolicyReference: row.PolicyReference, PolicyVersion: row.PolicyVersion,
			ActionCode: row.ActionCode, Locale: row.Locale, OccurredAt: row.OccurredAt.UTC(),
		})
	}
	return result, nil
}

// WithdrawCurrentPrivacyConsent appends a withdrawal and leaves every prior
// grant, contract, tax, dispute, audit, and legal-hold record unchanged.
func (s *serviceImpl) WithdrawCurrentPrivacyConsent(
	ctx context.Context,
	input identitycap.PrivacyConsentWithdrawalInput,
) (*identitycap.PrivacyConsentDecision, error) {
	if s == nil || ctx == nil || ctx.Err() != nil ||
		(input.ConsentType != identitycap.PrivacyConsentSensitivePersonal &&
			input.ConsentType != identitycap.PrivacyConsentEnterprise) ||
		!validConsentIdempotencyKey(input.IdempotencyKey) {
		return nil, gerror.New("privacy consent withdrawal input is invalid")
	}
	subject, err := s.CurrentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	consentType := string(input.ConsentType)
	numberType, purpose := personalConsentNumberType, "personal-consent-withdrawal"
	if input.ConsentType == identitycap.PrivacyConsentEnterprise {
		numberType, purpose = enterpriseConsentNumberType, "enterprise-consent-withdrawal"
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	var result *identitycap.PrivacyConsentDecision
	err = dao.CustomerConsent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var latest *entity.CustomerConsent
		if loadErr := dao.CustomerConsent.Ctx(ctx).Where(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo,
			ConsentTypeCode: consentType,
		}).OrderDesc("occurred_at").OrderDesc("id").LockUpdate().Limit(1).Scan(&latest); loadErr != nil {
			return loadErr
		}
		if latest == nil {
			return gerror.New("privacy consent is not accessible")
		}
		consentNo, reserveErr := s.reserveConsentNumber(
			ctx, numberType, purpose, subject.CustomerNo, input.IdempotencyKey,
		)
		if reserveErr != nil {
			return reserveErr
		}
		var existing *entity.CustomerConsent
		if loadErr := dao.CustomerConsent.Ctx(ctx).Where(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, ConsentNo: consentNo,
			CustomerNo: subject.CustomerNo,
		}).Limit(1).Scan(&existing); loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.ConsentTypeCode != consentType || existing.ActionCode != consentWithdrawn ||
				existing.PolicyReference != latest.PolicyReference || existing.PolicyVersion != latest.PolicyVersion ||
				existing.RecordedBy != subject.ActorUserID {
				return gerror.New("privacy consent withdrawal idempotency conflict")
			}
			result = privacyConsentDecision(existing, true)
			return nil
		}
		if latest.ActionCode == consentWithdrawn {
			return gerror.New("privacy consent is already withdrawn")
		}
		evidence := s.personalConsentDigest(
			purpose+"-evidence-v1",
			subject.CustomerNo+"\x00"+consentNo+"\x00"+latest.PolicyReference+"\x00"+
				latest.PolicyVersion+"\x00"+latest.Locale+"\x00"+strconv.Itoa(subject.ActorUserID),
		)
		rowID, insertErr := dao.CustomerConsent.Ctx(ctx).Data(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, ConsentNo: consentNo, CustomerNo: subject.CustomerNo,
			ConsentTypeCode: consentType, PolicyReference: latest.PolicyReference,
			PolicyVersion: latest.PolicyVersion, ActionCode: consentWithdrawn,
			SourceCode: personalConsentSource, Locale: latest.Locale,
			EvidenceDigest: evidence, OccurredAt: &now, RecordedBy: subject.ActorUserID,
		}).InsertAndGetId()
		if insertErr != nil {
			return insertErr
		}
		if assignErr := assignPersonalConsentNumber(ctx, consentNo, rowID, now); assignErr != nil {
			return assignErr
		}
		result = &identitycap.PrivacyConsentDecision{
			ConsentNo: consentNo, ConsentType: input.ConsentType,
			PolicyReference: latest.PolicyReference, PolicyVersion: latest.PolicyVersion,
			ActionCode: consentWithdrawn, Locale: latest.Locale, OccurredAt: now,
		}
		return nil
	})
	return result, err
}

func privacyConsentDecision(
	row *entity.CustomerConsent,
	reused bool,
) *identitycap.PrivacyConsentDecision {
	if row == nil || row.OccurredAt == nil {
		return nil
	}
	return &identitycap.PrivacyConsentDecision{
		ConsentNo: row.ConsentNo, ConsentType: identitycap.PrivacyConsentType(row.ConsentTypeCode),
		PolicyReference: row.PolicyReference, PolicyVersion: row.PolicyVersion,
		ActionCode: row.ActionCode, Locale: row.Locale, OccurredAt: row.OccurredAt.UTC(), Reused: reused,
	}
}
