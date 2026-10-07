package contractsigner

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-notify/backend/cap/securitycodecap"
)

func (s *serviceImpl) CreatePhoneChallenge(ctx context.Context, signerNo string, browserNonce []byte,
	locale, presentationKind string) (*securitycodecap.HumanChallenge, error) {
	defer clear(browserNonce)
	row, profile, err := s.currentProfile(ctx, signerNo)
	if err != nil {
		return nil, err
	}
	defer profile.clear()
	return s.securityCodes.CreateHumanChallenge(ctx, securitycodecap.HumanChallengeInput{Binding: securitycodecap.Binding{
		Purpose: PhonePurpose, SubjectNo: row.SignerNo, BrowserNonce: browserNonce, Destination: profile.phone, Locale: locale},
		PresentationKind: presentationKind})
}

func (s *serviceImpl) CompletePhoneChallenge(ctx context.Context, signerNo string, browserNonce []byte, locale,
	challengeNo string, answer, providerToken []byte) (*securitycodecap.HumanProof, error) {
	defer clear(browserNonce)
	defer clear(answer)
	defer clear(providerToken)
	row, profile, err := s.currentProfile(ctx, signerNo)
	if err != nil {
		return nil, err
	}
	defer profile.clear()
	return s.securityCodes.CompleteHumanChallenge(ctx, securitycodecap.HumanCompletionInput{Binding: securitycodecap.Binding{
		Purpose: PhonePurpose, SubjectNo: row.SignerNo, BrowserNonce: browserNonce, Destination: profile.phone, Locale: locale},
		ChallengeNo: challengeNo, Answer: answer, ProviderToken: providerToken})
}

func (s *serviceImpl) SendPhoneCode(ctx context.Context, signerNo string, browserNonce []byte, locale, proofToken,
	idempotencyKey string, clientIP []byte) (*securitycodecap.Verification, error) {
	defer clear(browserNonce)
	defer clear(clientIP)
	row, profile, err := s.currentProfile(ctx, signerNo)
	if err != nil {
		return nil, err
	}
	defer profile.clear()
	verification, err := s.securityCodes.Send(ctx, securitycodecap.SendInput{Binding: securitycodecap.Binding{Purpose: PhonePurpose,
		SubjectNo: row.SignerNo, BrowserNonce: browserNonce, Destination: profile.phone, Locale: locale}, ProofToken: proofToken,
		IdempotencyKey: idempotencyKey, ClientIP: clientIP})
	if err != nil {
		return nil, err
	}
	if verification == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_contract_signer_profile
SET phone_status_code='verifying',phone_verification_no=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND signer_no=? AND customer_no=? AND status_code='active' AND phone_status_code<>'verified' AND version=?`,
		verification.VerificationNo, idccap.DefaultTenantID, row.SignerNo, row.CustomerNo, row.Version)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	count, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return nil, bizerr.WrapCode(rowsErr, CodeUnavailable)
	}
	if count != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return verification, nil
}

func (s *serviceImpl) VerifyPhone(ctx context.Context, signerNo string, browserNonce []byte, locale, verificationNo string, code []byte) (*Signer, error) {
	defer clear(browserNonce)
	defer clear(code)
	row, profile, err := s.currentProfile(ctx, signerNo)
	if err != nil {
		return nil, err
	}
	defer profile.clear()
	verified, err := s.securityCodes.Verify(ctx, securitycodecap.VerifyInput{Binding: securitycodecap.Binding{Purpose: PhonePurpose,
		SubjectNo: row.SignerNo, BrowserNonce: browserNonce, Destination: profile.phone, Locale: locale}, VerificationNo: verificationNo, Code: code})
	if err != nil {
		return nil, err
	}
	if verified == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		result, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_profile
SET phone_status_code='verified',phone_verification_no=?,phone_verification_version=?,phone_verified_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND signer_no=? AND customer_no=? AND status_code='active' AND phone_status_code<>'verified'`, verificationNo,
			verified.Version, verified.VerifiedAt, idccap.DefaultTenantID, row.SignerNo, row.CustomerNo)
		if updateErr != nil {
			return updateErr
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			current, loadErr := s.loadOwned(txCtx, row.CustomerNo, row.SignerNo)
			if loadErr != nil {
				return loadErr
			}
			if current == nil || current.PhoneVerificationNo == nil || *current.PhoneVerificationNo != verificationNo {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "compliance.contract-signer.phone-verified",
			IdempotencyKey: verificationNo, OperationCode: "compliance.contract-signer.phone-verify", TargetType: "contract_signer", TargetNo: row.SignerNo,
			CustomerNo: row.CustomerNo, ResultCode: "succeeded", ReasonCode: "notify_security_code_verified", RiskCode: "high",
			AfterSnapshot: map[string]any{"phone_status": "verified", "phone_verification_version": verified.Version}})
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	row, err = s.loadOwned(ctx, row.CustomerNo, row.SignerNo)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	item := projectSigner(*row)
	return &item, nil
}

func (s *serviceImpl) currentProfile(ctx context.Context, signerNo string) (*signerRow, *privateProfile, error) {
	if !validBusinessNo(signerNo) {
		return nil, nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	row, err := s.loadOwned(ctx, subject.CustomerNo, signerNo)
	if err != nil || row == nil || row.StatusCode != "active" {
		return nil, nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	var copied *privateProfile
	err = s.accessProfile(ctx, *row, func(profile *privateProfile) error {
		copied = &privateProfile{name: append([]byte(nil), profile.name...), certNo: append([]byte(nil), profile.certNo...), phone: append([]byte(nil), profile.phone...)}
		return nil
	})
	if err != nil || copied == nil {
		return nil, nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return row, copied, nil
}
