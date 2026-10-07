package contractsigner

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type identityAttemptRow struct {
	ID                      int64      `orm:"id"`
	AttemptNo               string     `orm:"attempt_no"`
	SignerNo                string     `orm:"signer_no"`
	OuterOrderNo            string     `orm:"outer_order_no"`
	ConfigurationNo         string     `orm:"configuration_no"`
	ConfigurationVersion    int64      `orm:"configuration_version"`
	CredentialGeneration    int64      `orm:"credential_generation"`
	CertifyID               *string    `orm:"certify_id"`
	StatusCode              string     `orm:"status_code"`
	ReasonCode              string     `orm:"reason_code"`
	InitializationStartedAt time.Time  `orm:"initialization_started_at"`
	CertifyExpiresAt        *time.Time `orm:"certify_expires_at"`
	LastQueriedAt           *time.Time `orm:"last_queried_at"`
	PassedAt                *time.Time `orm:"passed_at"`
	ExpiredAt               *time.Time `orm:"expired_at"`
	Version                 int64      `orm:"version"`
}

func (s *serviceImpl) StartIdentity(ctx context.Context, signerNo, returnURL string) (*IdentityStart, error) {
	if !validBusinessNo(signerNo) || !validReturnURL(returnURL) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	row, err := s.loadOwned(ctx, subject.CustomerNo, signerNo)
	if err != nil || row == nil || row.StatusCode != "active" {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if row.IdentityStatusCode == "verified" {
		return nil, bizerr.NewCode(CodeConflict)
	}
	configuration, err := s.configurations.Resolve(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	nonce, err := randomBytes(16)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	defer clear(nonce)
	attemptNo := "csia_" + digest(signerNo, string(nonce), s.now().UTC().Format(time.RFC3339Nano))[:32]
	outerOrderNo := "CSIA" + digest(attemptNo)[:28]
	startedAt := s.now().UTC().Truncate(time.Microsecond)
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		result, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_profile
SET identity_status_code='verifying',identity_attempt_no=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND signer_no=? AND customer_no=? AND status_code='active' AND identity_status_code<>'verified' AND version=?`,
			attemptNo, idccap.DefaultTenantID, signerNo, subject.CustomerNo, row.Version)
		if updateErr != nil {
			return updateErr
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		_, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_compliance_contract_signer_identity_attempt
(tenant_id,attempt_no,signer_no,outer_order_no,configuration_no,configuration_version,credential_generation,status_code,reason_code,initialization_started_at,version)
VALUES (?,?,?,?,?,?,?,'preparing','',?,1)`, idccap.DefaultTenantID, attemptNo, signerNo, outerOrderNo,
			configuration.ConfigurationNo, configuration.Version, configuration.CredentialGeneration, startedAt)
		return insertErr
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeConflict)
	}
	client, err := s.clients.Create(configuration)
	if err != nil {
		if persistenceErr := s.markIdentityReview(ctx, signerNo, attemptNo, "provider_client_unavailable"); persistenceErr != nil {
			return nil, bizerr.WrapCode(errors.Join(err, persistenceErr), CodeUnavailable)
		}
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	var initiation aliyunface.Initiation
	err = s.accessProfile(ctx, *row, func(profile *privateProfile) error {
		result, callErr := client.Initiate(ctx, aliyunface.InitiateRequest{SceneID: configuration.SceneID, OuterOrderNo: outerOrderNo,
			CertType: aliyunface.CertTypeIdentityCard, CertName: string(profile.name), CertNo: string(profile.certNo), ReturnURL: returnURL})
		initiation = result
		return callErr
	})
	if err != nil {
		reason := "provider_initialization_rejected"
		var callErr *aliyunface.CallError
		if errors.As(err, &callErr) && callErr.Unknown {
			reason = "provider_initialization_unknown"
		}
		if persistenceErr := s.markIdentityReview(ctx, signerNo, attemptNo, reason); persistenceErr != nil {
			return nil, bizerr.WrapCode(errors.Join(err, persistenceErr), CodeUnavailable)
		}
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	expiresAt := startedAt.Add(aliyunface.CertificationLifetime)
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		attemptResult, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_identity_attempt
SET certify_id=?,status_code='pending',certify_expires_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND attempt_no=? AND status_code='preparing'`, initiation.CertifyID, expiresAt, idccap.DefaultTenantID, attemptNo)
		if updateErr != nil {
			return updateErr
		}
		count, _ := attemptResult.RowsAffected()
		if count != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "compliance.contract-signer.identity-start",
			IdempotencyKey: attemptNo, OperationCode: "compliance.contract-signer.identity-start", TargetType: "contract_signer", TargetNo: signerNo,
			CustomerNo: subject.CustomerNo, ResultCode: "succeeded", ReasonCode: "enterprise_signer_identity_started", RiskCode: "high",
			AfterSnapshot: map[string]any{"attempt_no": attemptNo, "status": "verifying"}})
	})
	if err != nil {
		persistenceErr := s.markIdentityReview(ctx, signerNo, attemptNo, "provider_result_persistence_failed")
		return nil, bizerr.WrapCode(errors.Join(err, persistenceErr), CodeUnavailable)
	}
	return &IdentityStart{SignerNo: signerNo, AttemptNo: attemptNo, Status: "verifying", ContinuationURL: initiation.CertifyURL, ExpiresAt: &expiresAt}, nil
}

func (s *serviceImpl) QueryIdentity(ctx context.Context, signerNo, attemptNo string) (*Signer, error) {
	if !validBusinessNo(signerNo) || !validBusinessNo(attemptNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	row, err := s.loadOwned(ctx, subject.CustomerNo, signerNo)
	if err != nil || row == nil || row.IdentityAttemptNo == nil || *row.IdentityAttemptNo != attemptNo {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	attempt, err := s.loadAttempt(ctx, signerNo, attemptNo)
	if err != nil || attempt == nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if attempt.StatusCode == "passed" || row.IdentityStatusCode == "verified" {
		item := projectSigner(*row)
		return &item, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if attempt.CertifyExpiresAt == nil || !now.Before(attempt.CertifyExpiresAt.UTC()) {
		if err = s.markIdentityExpired(ctx, *row, *attempt, now); err != nil {
			return nil, bizerr.WrapCode(err, CodeUnavailable)
		}
		row, _ = s.loadOwned(ctx, subject.CustomerNo, signerNo)
		item := projectSigner(*row)
		return &item, nil
	}
	if attempt.StatusCode != "pending" || attempt.CertifyID == nil {
		item := projectSigner(*row)
		return &item, nil
	}
	configuration, err := s.configurations.Resolve(ctx)
	if err != nil || configuration.ConfigurationNo != attempt.ConfigurationNo || configuration.Version != attempt.ConfigurationVersion || configuration.CredentialGeneration != attempt.CredentialGeneration {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	client, err := s.clients.Create(configuration)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	result, err := client.Query(ctx, aliyunface.QueryRequest{SceneID: configuration.SceneID, CertifyID: *attempt.CertifyID})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	switch result.State {
	case aliyunface.QueryStatePassed:
		err = s.markIdentityPassed(ctx, *row, *attempt, now)
	case aliyunface.QueryStateExpired:
		err = s.markIdentityExpired(ctx, *row, *attempt, now)
	case aliyunface.QueryStateNotPassed:
		_, err = s.database.Exec(ctx, `UPDATE plugin_idc_compliance_contract_signer_identity_attempt SET last_queried_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND attempt_no=? AND status_code='pending'`, now, idccap.DefaultTenantID, attemptNo)
	default:
		err = bizerr.NewCode(CodeUnavailable)
	}
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	row, err = s.loadOwned(ctx, subject.CustomerNo, signerNo)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	item := projectSigner(*row)
	return &item, nil
}

func (s *serviceImpl) loadAttempt(ctx context.Context, signerNo, attemptNo string) (*identityAttemptRow, error) {
	var row identityAttemptRow
	if err := s.database.Model(attemptTable).Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).Where("signer_no", signerNo).Where("attempt_no", attemptNo).Limit(1).Scan(&row); err != nil {
		return nil, err
	}
	if row.AttemptNo == "" {
		return nil, nil
	}
	return &row, nil
}

func (s *serviceImpl) markIdentityReview(ctx context.Context, signerNo, attemptNo, reason string) error {
	return s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_identity_attempt SET status_code='review_required',reason_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND attempt_no=? AND status_code IN ('preparing','pending')`, reason, idccap.DefaultTenantID, attemptNo); err != nil {
			return err
		}
		_, err := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_profile SET identity_status_code='review_required',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND signer_no=? AND identity_attempt_no=? AND identity_status_code='verifying'`, idccap.DefaultTenantID, signerNo, attemptNo)
		return err
	})
}

func (s *serviceImpl) markIdentityPassed(ctx context.Context, row signerRow, attempt identityAttemptRow, now time.Time) error {
	return s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		result, err := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_identity_attempt SET status_code='passed',passed_at=?,last_queried_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND attempt_no=? AND status_code='pending' AND version=?`, now, now, idccap.DefaultTenantID, attempt.AttemptNo, attempt.Version)
		if err != nil {
			return err
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		result, err = s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_profile SET identity_status_code='verified',identity_verification_version=identity_verification_version+1,identity_verified_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND signer_no=? AND identity_attempt_no=? AND status_code='active' AND version=?`, now, idccap.DefaultTenantID, row.SignerNo, attempt.AttemptNo, row.Version)
		if err != nil {
			return err
		}
		count, _ = result.RowsAffected()
		if count != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "compliance.contract-signer.identity-verified", IdempotencyKey: attempt.AttemptNo, OperationCode: "compliance.contract-signer.identity-verify", TargetType: "contract_signer", TargetNo: row.SignerNo, CustomerNo: row.CustomerNo, ResultCode: "succeeded", ReasonCode: "aliyun_authoritative_passed", RiskCode: "high", AfterSnapshot: map[string]any{"identity_status": "verified", "identity_verification_version": row.IdentityVerificationVersion + 1}})
	})
}

func (s *serviceImpl) markIdentityExpired(ctx context.Context, row signerRow, attempt identityAttemptRow, now time.Time) error {
	return s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_identity_attempt SET status_code='expired',expired_at=?,last_queried_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND attempt_no=? AND status_code='pending'`, now, now, idccap.DefaultTenantID, attempt.AttemptNo); err != nil {
			return err
		}
		_, err := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_profile SET identity_status_code='expired',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND signer_no=? AND identity_attempt_no=? AND identity_status_code='verifying'`, idccap.DefaultTenantID, row.SignerNo, attempt.AttemptNo)
		return err
	})
}

func validReturnURL(value string) bool {
	if len(value) < 9 || len(value) > 2048 || value != strings.TrimSpace(value) ||
		strings.ContainsAny(value, "\r\n\t\x00") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" &&
		parsed.User == nil && parsed.Fragment == ""
}
