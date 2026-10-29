// This file governs supplier-subject binding, revocation, and reopening workflows.

package supplierconfig

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const (
	auditSubjectGateOperation     = "compliance.local-supplier-subject.gate"
	auditSubjectPendingOperation  = "compliance.local-supplier-subject.pending"
	auditSubjectCompleteOperation = "compliance.local-supplier-subject.complete"
	supplierReauthorizedReason    = "supplier_reauthorized"
)

func (s *serviceImpl) StartSubjectBinding(ctx context.Context, provider string) error {
	return s.requireSubjectRecognizer(ctx, provider, OperationSubjectBind, PermissionSubjectBind)
}

func (s *serviceImpl) StartSubjectReopening(ctx context.Context, provider string) error {
	return s.requireSubjectRecognizer(ctx, provider, OperationSubjectReopen, PermissionSubjectReopen)
}

func (s *serviceImpl) requireSubjectRecognizer(
	ctx context.Context,
	provider string,
	operation OperationKind,
	permission string,
) error {
	provider = normalizeProvider(provider)
	actor, err := s.authorize(ctx, permission, provider)
	if err != nil {
		return err
	}
	if err = s.ensureRecentLogin(ctx, actor); err != nil {
		return err
	}
	if s.gateway.RecognizerAvailable(provider) {
		return nil
	}
	return s.recordUnavailableRecognizer(ctx, provider, operation, permission)
}

func (s *serviceImpl) recordUnavailableRecognizer(
	ctx context.Context,
	provider string,
	operation OperationKind,
	permission string,
) error {
	// The external gate is intentionally evaluated before any configuration
	// read or write so an unavailable recognizer cannot create pending state.
	snapshot := auditSnapshot(nil)
	auditKey, err := newMutationOperationKey()
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if err := s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope:   auditSubjectGateOperation,
		IdempotencyKey:     auditKey,
		OperationCode:      auditSubjectGateOperation,
		TargetType:         auditTargetType,
		TargetNo:           configurationNo(provider),
		ResultCode:         "denied",
		ReasonCode:         EvidenceExternalConfirmationRequired,
		RiskCode:           "high",
		RequiredPermission: permission,
		BeforeSnapshot:     snapshot,
		AfterSnapshot:      snapshot,
	}); err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	return bizerr.NewCode(CodeExternalEvidenceRequired)
}

func (s *serviceImpl) IssueSubjectAuthorization(
	ctx context.Context,
	request SubjectAuthorizationRequest,
) (*SubjectAuthorizationContext, error) {
	request, permission, err := normalizeSubjectAuthorizationRequest(request)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, permission, request.ProviderCode)
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	if !s.gateway.RecognizerAvailable(request.ProviderCode) {
		return nil, s.recordUnavailableRecognizer(
			ctx, request.ProviderCode, request.Operation, permission,
		)
	}
	if err = s.validateSubjectStart(ctx, request); err != nil {
		return nil, err
	}
	binding := subjectProofBinding(request)
	issued, err := s.runtime.SensitiveAccess.IssueSensitiveAccess(ctx, binding)
	if err != nil || issued == nil || strings.TrimSpace(issued.Token) == "" || issued.ExpiresAt.IsZero() {
		return nil, bizerr.WrapCode(firstError(err, bizerr.NewCode(CodeRecentAuthenticationRequired)), CodeRecentAuthenticationRequired)
	}
	if err = s.markSubjectPending(ctx, request, issued.Token, issued.ExpiresAt); err != nil {
		return nil, err
	}
	return &SubjectAuthorizationContext{Token: issued.Token, ExpiresAt: issued.ExpiresAt.UTC()}, nil
}

func (s *serviceImpl) CompleteSubjectAuthorization(
	ctx context.Context,
	result SubjectAuthorizationResult,
) (*Configuration, error) {
	defer clearBytes(result.AuthorizationResult)
	owned := append([]byte(nil), result.AuthorizationResult...)
	defer clearBytes(owned)
	request, _, err := normalizeSubjectAuthorizationRequest(result.Request)
	if err != nil || strings.TrimSpace(result.ProofToken) == "" || len(owned) == 0 || len(owned) > 4096 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if err = s.validateSubjectCompletion(ctx, request, result.ProofToken); err != nil {
		return nil, err
	}
	proof, err := s.runtime.SensitiveAccess.ConsumeSensitiveAccess(
		ctx, result.ProofToken, subjectProofBinding(request),
	)
	if err != nil || proof == nil || proof.ActorUserID <= 0 {
		return nil, bizerr.NewCode(CodeRecentAuthenticationRequired)
	}
	var gatewayResult GatewayResult
	switch request.Operation {
	case OperationSubjectBind:
		gatewayResult, err = s.gateway.Bind(
			ctx, request.ProviderCode, owned, request.IdempotencyKey,
		)
	case OperationSubjectReopen:
		employeePrincipalNo := "staff:" + strconv.Itoa(proof.ActorUserID)
		reopeningEvidence := sha256.Sum256([]byte(strings.Join([]string{
			"idc-supplier-authorization-reopening-v1", request.ProviderCode,
			request.PreviousBindingNo, supplierReauthorizedReason,
			strconv.FormatInt(request.ExpectedVersion, 10), employeePrincipalNo,
			request.IdempotencyKey,
		}, "\x00")))
		gatewayResult, err = s.gateway.Reopen(
			ctx, request.ProviderCode, request.PreviousBindingNo,
			supplierReauthorizedReason, owned, employeePrincipalNo,
			hex.EncodeToString(reopeningEvidence[:]), request.IdempotencyKey,
		)
	default:
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if err != nil {
		if markErr := s.markSubjectFailed(ctx, request); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return nil, bizerr.WrapCode(err, CodeGatewayUnavailable)
	}
	if !operationNoPattern.MatchString(gatewayResult.OperationNo) ||
		!bindingNoPattern.MatchString(gatewayResult.BindingNo) {
		if markErr := s.markSubjectFailed(ctx, request); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return nil, bizerr.NewCode(CodeGatewayUnavailable)
	}
	if err = s.completeSubjectProjection(ctx, request, gatewayResult); err != nil {
		return nil, err
	}
	return s.currentConfiguration(ctx, request.ProviderCode)
}

func (s *serviceImpl) RevokeSubject(
	ctx context.Context,
	input SubjectRevokeInput,
) (*Configuration, error) {
	input.ProviderCode = normalizeProvider(input.ProviderCode)
	input.BindingNo = strings.TrimSpace(input.BindingNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validProvider(input.ProviderCode) || !bindingNoPattern.MatchString(input.BindingNo) ||
		!validRevokeReason(input.ReasonCode) || input.ExpectedVersion <= 0 ||
		!idempotencyKeyPattern.MatchString(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionSubjectRevoke, input.ProviderCode)
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	row, err := s.load(ctx, input.ProviderCode, false)
	status := SubjectStatus("")
	if row != nil {
		status = SubjectStatus(row.SubjectStatusCode)
	}
	if err != nil || row == nil || row.Version != input.ExpectedVersion ||
		(status != SubjectActive &&
			!(status == SubjectRetryableFailed &&
				OperationKind(row.LastOperationKindCode) == OperationSubjectRevoke)) ||
		safeString(row.SubjectBindingNo) != input.BindingNo {
		return nil, firstError(err, bizerr.NewCode(CodeConflict))
	}
	employeePrincipalNo := "staff:" + strconv.Itoa(actor)
	revocationEvidence := sha256.Sum256([]byte(strings.Join([]string{
		"idc-supplier-authorization-revocation-v1", input.ProviderCode,
		input.BindingNo, input.ReasonCode, strconv.FormatInt(input.ExpectedVersion, 10),
		employeePrincipalNo, input.IdempotencyKey,
	}, "\x00")))
	result, err := s.gateway.Revoke(
		ctx, input.ProviderCode, input.BindingNo, input.ReasonCode,
		employeePrincipalNo, hex.EncodeToString(revocationEvidence[:]), input.IdempotencyKey,
	)
	request := SubjectAuthorizationRequest{
		ProviderCode: input.ProviderCode, Operation: OperationSubjectRevoke,
		PreviousBindingNo: input.BindingNo, ExpectedVersion: input.ExpectedVersion,
		IdempotencyKey: input.IdempotencyKey,
	}
	if err != nil {
		if markErr := s.markSubjectFailed(ctx, request); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return nil, bizerr.WrapCode(err, CodeGatewayUnavailable)
	}
	if !operationNoPattern.MatchString(result.OperationNo) || result.BindingNo != input.BindingNo {
		if markErr := s.markSubjectFailed(ctx, request); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return nil, bizerr.NewCode(CodeGatewayUnavailable)
	}
	if err = s.completeSubjectProjection(ctx, request, result); err != nil {
		return nil, err
	}
	return s.currentConfiguration(ctx, input.ProviderCode)
}

func normalizeSubjectAuthorizationRequest(
	request SubjectAuthorizationRequest,
) (SubjectAuthorizationRequest, string, error) {
	request.ProviderCode = normalizeProvider(request.ProviderCode)
	request.PreviousBindingNo = strings.TrimSpace(request.PreviousBindingNo)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if !validProvider(request.ProviderCode) || request.ExpectedVersion < 0 ||
		request.ExpectedVersion == math.MaxInt64 ||
		!idempotencyKeyPattern.MatchString(request.IdempotencyKey) {
		return SubjectAuthorizationRequest{}, "", bizerr.NewCode(CodeInvalid)
	}
	switch request.Operation {
	case OperationSubjectBind:
		if request.PreviousBindingNo != "" {
			return SubjectAuthorizationRequest{}, "", bizerr.NewCode(CodeInvalid)
		}
		return request, PermissionSubjectBind, nil
	case OperationSubjectReopen:
		if !bindingNoPattern.MatchString(request.PreviousBindingNo) {
			return SubjectAuthorizationRequest{}, "", bizerr.NewCode(CodeInvalid)
		}
		return request, PermissionSubjectReopen, nil
	default:
		return SubjectAuthorizationRequest{}, "", bizerr.NewCode(CodeInvalid)
	}
}

func subjectProofBinding(request SubjectAuthorizationRequest) managementcap.SensitiveAccessBinding {
	canonical := strings.Join([]string{
		"idc-compliance/supplier-subject/v1", request.ProviderCode,
		string(request.Operation), request.PreviousBindingNo,
		strconv.FormatInt(request.ExpectedVersion, 10), strings.TrimSpace(request.IdempotencyKey),
	}, "\x00")
	return managementcap.SensitiveAccessBinding{
		TargetDigest: managementcap.DigestSensitiveAccessTarget([]byte(canonical)),
	}
}

func (s *serviceImpl) validateSubjectStart(
	ctx context.Context,
	request SubjectAuthorizationRequest,
) error {
	row, err := s.load(ctx, request.ProviderCode, false)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		if request.Operation == OperationSubjectBind && request.ExpectedVersion == 0 {
			return nil
		}
		return bizerr.NewCode(CodeConflict)
	}
	if row.Version != request.ExpectedVersion {
		return bizerr.NewCode(CodeConflict)
	}
	status := SubjectStatus(row.SubjectStatusCode)
	if request.Operation == OperationSubjectBind &&
		(status == SubjectUnbound ||
			(status == SubjectRetryableFailed && row.SubjectBindingNo == nil &&
				OperationKind(row.LastOperationKindCode) == OperationSubjectBind)) {
		return nil
	}
	if request.Operation == OperationSubjectReopen &&
		(status == SubjectRevoked ||
			(status == SubjectRetryableFailed &&
				OperationKind(row.LastOperationKindCode) == OperationSubjectReopen)) &&
		safeString(row.SubjectBindingNo) == request.PreviousBindingNo {
		return nil
	}
	return bizerr.NewCode(CodeConflict)
}

func (s *serviceImpl) markSubjectPending(
	ctx context.Context,
	request SubjectAuthorizationRequest,
	proofToken string,
	proofExpiresAt time.Time,
) error {
	return s.updateSubjectOperation(
		ctx, request, OperationResultPending, "", "",
		proofTokenDigest(proofToken), utcPointer(&proofExpiresAt),
	)
}

func (s *serviceImpl) markSubjectFailed(
	ctx context.Context,
	request SubjectAuthorizationRequest,
) error {
	return s.updateSubjectOperation(
		ctx, request, OperationResultRetryableFailed, "", "", "", nil,
	)
}

func (s *serviceImpl) completeSubjectProjection(
	ctx context.Context,
	request SubjectAuthorizationRequest,
	result GatewayResult,
) error {
	return s.updateSubjectOperation(
		ctx, request, OperationResultSucceeded, result.OperationNo, result.BindingNo, "", nil,
	)
}

func (s *serviceImpl) validateSubjectCompletion(
	ctx context.Context,
	request SubjectAuthorizationRequest,
	proofToken string,
) error {
	row, err := s.load(ctx, request.ProviderCode, false)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	expectedStatus := SubjectBinding
	if request.Operation == OperationSubjectReopen {
		expectedStatus = SubjectReopening
	}
	if row == nil || row.Version != request.ExpectedVersion+1 ||
		SubjectStatus(row.SubjectStatusCode) != expectedStatus ||
		OperationKind(row.LastOperationKindCode) != request.Operation ||
		OperationResult(row.LastOperationResultCode) != OperationResultPending ||
		row.SubjectProofDigest == nil || row.SubjectProofExpiresAt == nil ||
		!row.SubjectProofExpiresAt.After(s.now().UTC()) ||
		subtle.ConstantTimeCompare(
			[]byte(*row.SubjectProofDigest), []byte(proofTokenDigest(proofToken)),
		) != 1 {
		return bizerr.NewCode(CodeRecentAuthenticationRequired)
	}
	if request.Operation == OperationSubjectBind && row.SubjectBindingNo != nil {
		return bizerr.NewCode(CodeConflict)
	}
	if request.Operation == OperationSubjectReopen &&
		safeString(row.SubjectBindingNo) != request.PreviousBindingNo {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) updateSubjectOperation(
	ctx context.Context,
	request SubjectAuthorizationRequest,
	result OperationResult,
	operationNo string,
	bindingNo string,
	proofDigest string,
	proofExpiresAt *time.Time,
) error {
	now := s.now().UTC()
	evidenceCode, adapterEnabled := subjectAdapterGate(
		request.ProviderCode, request.Operation, result,
	)
	return publicError(dao.SupplierConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, request.ProviderCode, true)
		if err != nil {
			return err
		}
		before := row
		if row == nil {
			if request.Operation != OperationSubjectBind || request.ExpectedVersion != 0 {
				return bizerr.NewCode(CodeConflict)
			}
			_, err = dao.SupplierConfiguration.Ctx(ctx).Data(do.SupplierConfiguration{
				TenantId:                idccap.DefaultTenantID,
				ConfigurationNo:         configurationNo(request.ProviderCode),
				ProviderCode:            request.ProviderCode,
				CredentialStatusCode:    string(CredentialUnconfigured),
				SubjectStatusCode:       subjectStateFor(request.Operation, result),
				SubjectBindingNo:        nullableString(bindingNo),
				LastOperationKindCode:   string(request.Operation),
				LastOperationResultCode: string(result),
				LastOperationNo:         nullableString(operationNo), LastOperationAt: &now,
				SubjectProofDigest: nullableString(proofDigest), SubjectProofExpiresAt: proofExpiresAt,
				CapabilityEvidenceCode: evidenceCode,
				AdapterEnabled:         adapterEnabled, Version: int64(1),
			}).Insert()
		} else {
			expectedVersion := request.ExpectedVersion
			if result != OperationResultPending && request.Operation != OperationSubjectRevoke {
				expectedVersion++
			}
			if row.Version != expectedVersion {
				return bizerr.NewCode(CodeConflict)
			}
			resultSQL, updateErr := dao.SupplierConfiguration.Ctx(ctx).
				Where(do.SupplierConfiguration{Id: row.Id, Version: row.Version}).
				Data(do.SupplierConfiguration{
					SubjectStatusCode:       subjectStateFor(request.Operation, result),
					SubjectBindingNo:        nullableString(preferredBindingNo(bindingNo, row.SubjectBindingNo)),
					LastOperationKindCode:   string(request.Operation),
					LastOperationResultCode: string(result),
					LastOperationNo:         nullableString(operationNo), LastOperationAt: &now,
					SubjectProofDigest: nullableString(proofDigest), SubjectProofExpiresAt: proofExpiresAt,
					CapabilityEvidenceCode: evidenceCode, AdapterEnabled: adapterEnabled,
					Version: gdb.Raw(dao.SupplierConfiguration.Columns().Version + " + 1"),
				}).Update()
			if updateErr != nil {
				return updateErr
			}
			err = requireOneRow(resultSQL)
		}
		if err != nil {
			return err
		}
		after, err := s.load(ctx, request.ProviderCode, true)
		if err != nil || after == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		permission := permissionForOperation(request.Operation)
		auditOperation := auditSubjectCompleteOperation
		if result == OperationResultPending {
			auditOperation = auditSubjectPendingOperation
		}
		auditResult := idccap.BusinessAuditResultSucceeded
		if result == OperationResultRetryableFailed {
			auditResult = "failed"
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: auditOperation,
			IdempotencyKey:   subjectAuditKey(request, result),
			OperationCode:    auditOperation,
			TargetType:       auditTargetType, TargetNo: configurationNo(request.ProviderCode),
			ResultCode: auditResult, ReasonCode: string(request.Operation), RiskCode: "high",
			RequiredPermission: permission,
			BeforeSnapshot:     auditSnapshot(before), AfterSnapshot: auditSnapshot(after),
		})
	}))
}

func subjectAdapterGate(
	provider string,
	operation OperationKind,
	result OperationResult,
) (string, bool) {
	return EvidenceExternalConfirmationRequired, false
}

func subjectStateFor(operation OperationKind, result OperationResult) string {
	if result == OperationResultRetryableFailed {
		return string(SubjectRetryableFailed)
	}
	if result == OperationResultPending {
		if operation == OperationSubjectReopen {
			return string(SubjectReopening)
		}
		return string(SubjectBinding)
	}
	if operation == OperationSubjectRevoke {
		return string(SubjectRevoked)
	}
	return string(SubjectActive)
}

func permissionForOperation(operation OperationKind) string {
	switch operation {
	case OperationSubjectBind:
		return PermissionSubjectBind
	case OperationSubjectRevoke:
		return PermissionSubjectRevoke
	case OperationSubjectReopen:
		return PermissionSubjectReopen
	default:
		return PermissionQuery
	}
}

func subjectAuditKey(request SubjectAuthorizationRequest, result OperationResult) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		request.ProviderCode, string(request.Operation), request.PreviousBindingNo,
		strconv.FormatInt(request.ExpectedVersion, 10), request.IdempotencyKey, string(result),
	}, "\x00")))
	return hex.EncodeToString(digest[:])
}

func proofTokenDigest(token string) string {
	digest := sha256.Sum256([]byte("idc-compliance/subject-proof/v1\x00" + token))
	return hex.EncodeToString(digest[:])
}

func validRevokeReason(reason string) bool {
	switch reason {
	case "subject_replaced", "supplier_authorization_revoked", "security_compromise":
		return true
	default:
		return false
	}
}

func preferredBindingNo(newValue string, current *string) string {
	if newValue != "" {
		return newValue
	}
	return safeString(current)
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
