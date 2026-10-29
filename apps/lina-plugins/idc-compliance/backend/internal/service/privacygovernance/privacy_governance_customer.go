// Customer consent persistence exposes only the signed-in customer's current decisions and withdrawal.

package privacygovernance

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

func (s *serviceImpl) CurrentCustomerPrivacy(ctx context.Context) (*CustomerPrivacyOverview, error) {
	if nilDependency(s.customers) || nilDependency(s.consents) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	consents, err := s.consents.ListCurrentPrivacyConsents(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &CustomerPrivacyOverview{Consents: consents}, nil
}

func (s *serviceImpl) WithdrawCurrentCustomerConsent(
	ctx context.Context,
	input identitycap.PrivacyConsentWithdrawalInput,
) (*identitycap.PrivacyConsentDecision, error) {
	if nilDependency(s.customers) || nilDependency(s.consents) || nilDependency(s.customerAudit) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var result *identitycap.PrivacyConsentDecision
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		decision, withdrawErr := s.consents.WithdrawCurrentPrivacyConsent(ctx, input)
		if withdrawErr != nil {
			return withdrawErr
		}
		result = decision
		return s.customerAudit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.consent-withdraw", IdempotencyKey: sha256Hex("privacy-consent-withdraw\x00" + input.IdempotencyKey),
			OperationCode: "compliance.privacy.consent-withdraw", TargetType: "customer_privacy_consent",
			TargetNo: decision.ConsentNo, CustomerNo: subject.CustomerNo,
			ResultCode: "succeeded", ReasonCode: "future_processing_consent_withdrawn", RiskCode: "high",
			BeforeSnapshot: map[string]any{"consent_type": input.ConsentType, "action": "granted"},
			AfterSnapshot:  map[string]any{"consent_type": input.ConsentType, "action": "withdrawn", "historical_evidence_preserved": true},
		})
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}
