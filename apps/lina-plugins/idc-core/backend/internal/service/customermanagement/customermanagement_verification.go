// This file owns staff manual verification and current-profile clearing.
// Subject locks serialize with compliance projections; audit and changes share a transaction.
package customermanagement

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

// VerificationAction selects an explicit staff command, never a supplier decision.
type VerificationAction string

const (
	VerificationApprove      VerificationAction = "approve"
	VerificationClear        VerificationAction = "clear"
	verificationOperation                       = "customer.verification.update"
	verificationManualReason                    = "manual_verification_update"
)

// VerificationInput carries only the target, action, concurrency version and retry key.
type VerificationInput struct {
	CustomerNo      string
	Action          VerificationAction
	ExpectedVersion int64
	IdempotencyKey  string
}

// VerificationService is implemented by the existing shared mutation owner.
type VerificationService interface {
	// UpdateVerification checks staff permission and scope, then atomically applies or replays the command.
	// Clear removes current legal fields from both profile types and retains all historical evidence.
	UpdateVerification(context.Context, VerificationInput) error
}

var _ VerificationService = (*mutationServiceImpl)(nil)

// UpdateVerification never creates a fake case, identifier, or supplier response.
func (s *mutationServiceImpl) UpdateVerification(ctx context.Context, input VerificationInput) error {
	if err := authorizeStaff(ctx, s.bizCtxSvc, s.principal, s.authorization, permissionCustomerUpdate); err != nil {
		return err
	}
	if !validPersistedCustomerNo(input.CustomerNo) || (input.Action != VerificationApprove && input.Action != VerificationClear) || input.ExpectedVersion < 1 || !validActivityIdempotencyKey(input.IdempotencyKey) {
		return bizerr.NewCode(customer.CodeProfileInvalid)
	}
	err := dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var subject *entity.CustomerSubject
		if err := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo}).LockUpdate().Scan(&subject); err != nil {
			return err
		}
		if subject == nil || subject.StatusCode == string(customer.StatusAnonymized) {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		evidence, err := s.auditWriter.AppendBusinessBatchTransition(ctx, idccap.BusinessAuditBatchTransition{
			IdempotencyScope: verificationOperation, IdempotencyKey: input.IdempotencyKey, OperationCode: verificationOperation,
			TargetType: segmentAuditTarget, TargetNo: input.CustomerNo, AffectedCustomerNos: []string{input.CustomerNo},
			ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: verificationManualReason, RiskCode: segmentAuditRisk, RequiredPermission: permissionCustomerUpdate, CorrelationNo: input.CustomerNo,
			BeforeSnapshot: struct{ ExpectedVersion int64 }{input.ExpectedVersion}, AfterSnapshot: struct{ Action VerificationAction }{input.Action},
		})
		if err != nil {
			return err
		}
		if evidence.Replayed {
			return nil
		}
		if subject.Version != input.ExpectedVersion {
			return bizerr.NewCode(customer.CodeVersionConflict)
		}
		if err = updateVerificationProfiles(ctx, subject, input.Action); err != nil {
			return err
		}
		result, err := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{Id: subject.Id, TenantId: idccap.DefaultTenantID, Version: subject.Version}).Data(do.CustomerSubject{Version: subject.Version + 1}).Update()
		return verificationAffected(result, err)
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return err
		}
		return bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return nil
}

func updateVerificationProfiles(ctx context.Context, subject *entity.CustomerSubject, action VerificationAction) error {
	var (
		personal   *entity.CustomerIndividualProfile
		enterprise *entity.CustomerEnterpriseProfile
	)
	condition := do.CustomerIndividualProfile{TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo}
	if err := dao.CustomerIndividualProfile.Ctx(ctx).Where(condition).LockUpdate().Scan(&personal); err != nil {
		return err
	}
	if err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo}).LockUpdate().Scan(&enterprise); err != nil {
		return err
	}
	if action == VerificationApprove {
		now := time.Now().UTC()
		if subject.CustomerTypeCode == string(customer.TypeIndividual) && personal != nil {
			result, err := dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{Id: personal.Id, TenantId: idccap.DefaultTenantID, Version: personal.Version}).Data(do.CustomerIndividualProfile{IdentityStatusCode: customer.VerificationVerified, ManualVerification: true, IdentityVerifiedAt: &now, Version: personal.Version + 1}).Update()
			if err = verificationAffected(result, err); err != nil {
				return err
			}
			_, err = dao.CustomerIndividualProfile.Ctx(ctx).Where(condition).Data("identity_expires_at", gdb.Raw("NULL")).Update()
			return err
		}
		if subject.CustomerTypeCode == string(customer.TypeEnterprise) && enterprise != nil {
			result, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{Id: enterprise.Id, TenantId: idccap.DefaultTenantID, Version: enterprise.Version}).Data(do.CustomerEnterpriseProfile{VerificationStatusCode: customer.VerificationVerified, ManualVerification: true, VerifiedAt: &now, Version: enterprise.Version + 1}).Update()
			if err = verificationAffected(result, err); err != nil {
				return err
			}
			_, err = dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{Id: enterprise.Id, TenantId: idccap.DefaultTenantID}).Data("expires_at", gdb.Raw("NULL")).Update()
			return err
		}
		return bizerr.NewCode(customer.CodeProfileInvalid)
	}
	if personal != nil {
		result, err := dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{Id: personal.Id, TenantId: idccap.DefaultTenantID, Version: personal.Version}).Data(do.CustomerIndividualProfile{LegalName: "", LegalNameMasked: "", IdentityIdentifierMasked: "", IdentityLookupDigest: "", IdentityCaseNo: "", IdentityStatusCode: customer.VerificationUnverified, ManualVerification: true, Version: personal.Version + 1}).Update()
		if err = verificationAffected(result, err); err != nil {
			return err
		}
		if _, err = dao.CustomerIndividualProfile.Ctx(ctx).Where(condition).Data("identity_verified_at", gdb.Raw("NULL")).Update(); err != nil {
			return err
		}
		if _, err = dao.CustomerIndividualProfile.Ctx(ctx).Where(condition).Data("identity_expires_at", gdb.Raw("NULL")).Update(); err != nil {
			return err
		}
	}
	if enterprise != nil {
		condition := do.CustomerEnterpriseProfile{Id: enterprise.Id, TenantId: idccap.DefaultTenantID}
		result, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(condition).Where("version", enterprise.Version).Data(do.CustomerEnterpriseProfile{LegalName: "", RegistrationCountryCode: "CN", RegistrationIdentifier: "", RegistrationIdentifierMasked: "", RegistrationLookupDigest: "", IndustryCode: "", VerificationCaseNo: "", VerificationStatusCode: customer.VerificationUnverified, ManualVerification: true, Version: enterprise.Version + 1}).Update()
		if err = verificationAffected(result, err); err != nil {
			return err
		}
		if _, err = dao.CustomerEnterpriseProfile.Ctx(ctx).Where(condition).Data("verified_at", gdb.Raw("NULL")).Update(); err != nil {
			return err
		}
		if _, err = dao.CustomerEnterpriseProfile.Ctx(ctx).Where(condition).Data("expires_at", gdb.Raw("NULL")).Update(); err != nil {
			return err
		}
	}
	return nil
}

func verificationAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return bizerr.NewCode(customer.CodeVersionConflict)
	}
	return nil
}
