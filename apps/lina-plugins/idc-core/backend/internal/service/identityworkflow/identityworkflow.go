// Package identityworkflow adapts the core customer legal-change owner to the
// narrow source-only enterprise verification capability.
package identityworkflow

import (
	"context"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type serviceImpl struct {
	customers customer.OwnLegalChangeService
}

var _ identitycap.EnterpriseAccountService = (*serviceImpl)(nil)

// New creates the customer-owned enterprise account adapter.
func New(customers customer.OwnLegalChangeService) (identitycap.EnterpriseAccountService, error) {
	if nilDependency(customers) {
		return nil, gerror.New("enterprise identity workflow dependency is invalid")
	}
	return &serviceImpl{customers: customers}, nil
}

// CurrentEnterpriseAccount returns the current enterprise profile and at most
// one active legal-change request without accepting a customer identifier.
func (s *serviceImpl) CurrentEnterpriseAccount(
	ctx context.Context,
) (*identitycap.EnterpriseAccountState, error) {
	profile, err := s.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	if profile == nil || (profile.Subject.Type != customer.TypeIndividual &&
		profile.Subject.Type != customer.TypeEnterprise) || profile.Enterprise == nil {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	status, err := enterpriseStatus(profile.Enterprise.VerificationStatus)
	if err != nil {
		return nil, err
	}
	active, err := s.customers.CurrentOwnLegalChange(ctx)
	if err != nil {
		return nil, err
	}
	return &identitycap.EnterpriseAccountState{
		VerificationCaseNo:           profile.Enterprise.VerificationCaseNo,
		VerificationStatus:           status,
		LegalName:                    profile.Enterprise.LegalName,
		RegistrationCountryCode:      profile.Enterprise.RegistrationCountryCode,
		RegistrationIdentifierMasked: profile.Enterprise.RegistrationIdentifierMasked,
		IndustryCode:                 profile.Enterprise.IndustryCode,
		VerifiedAt:                   profile.Enterprise.VerifiedAt, ExpiresAt: profile.Enterprise.ExpiresAt,
		ProfileVersion:    profile.Enterprise.Version,
		ActiveLegalChange: mapLegalChange(active),
	}, nil
}

// RequestEnterpriseLegalChange delegates a category-only request to the core
// customer workflow after deriving the current customer from trusted context.
func (s *serviceImpl) RequestEnterpriseLegalChange(
	ctx context.Context,
	input identitycap.EnterpriseLegalChangeInput,
) (*identitycap.EnterpriseLegalChange, error) {
	profile, err := s.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	if profile == nil || profile.Subject.Type != customer.TypeEnterprise || profile.Enterprise == nil {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	fields := make([]customer.LegalChangeField, 0, len(input.Fields))
	for _, field := range input.Fields {
		mapped, mapErr := legalChangeField(field)
		if mapErr != nil {
			return nil, mapErr
		}
		fields = append(fields, mapped)
	}
	reason, err := legalChangeReason(input.Reason)
	if err != nil {
		return nil, err
	}
	result, err := s.customers.RequestOwnLegalChange(ctx, customer.RequestOwnLegalChangeInput{
		CustomerNo: profile.Subject.CustomerNo, Fields: fields, Reason: reason,
		ExpectedProfileVersion: input.ExpectedProfileVersion,
		IdempotencyKey:         input.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return mapLegalChange(result), nil
}

func enterpriseStatus(value customer.VerificationStatus) (identitycap.Status, error) {
	switch value {
	case customer.VerificationUnverified:
		return identitycap.StatusUnverified, nil
	case customer.VerificationPending:
		return identitycap.StatusPending, nil
	case customer.VerificationVerified:
		return identitycap.StatusVerified, nil
	case customer.VerificationRejected:
		return identitycap.StatusRejected, nil
	case customer.VerificationExpired:
		return identitycap.StatusExpired, nil
	case customer.VerificationRevoked:
		return identitycap.StatusRevoked, nil
	default:
		return "", gerror.New("enterprise verification status is invalid")
	}
}

func legalChangeField(value identitycap.EnterpriseLegalChangeField) (customer.LegalChangeField, error) {
	switch value {
	case identitycap.EnterpriseLegalNameField:
		return customer.LegalChangeFieldName, nil
	case identitycap.EnterpriseLegalIdentifierField:
		return customer.LegalChangeFieldIdentifier, nil
	default:
		return "", bizerr.NewCode(customer.CodeWorkflowInvalid)
	}
}

func legalChangeReason(value identitycap.EnterpriseLegalChangeReason) (customer.LegalChangeReason, error) {
	switch value {
	case identitycap.EnterpriseLegalNameChangeReason:
		return customer.LegalChangeReasonNameChange, nil
	case identitycap.EnterpriseIdentifierCorrectionReason:
		return customer.LegalChangeReasonIdentifierCorrection, nil
	case identitycap.EnterpriseRegistrationChangeReason:
		return customer.LegalChangeReasonEnterpriseRegistration, nil
	case identitycap.EnterpriseOtherLegalChangeReason:
		return customer.LegalChangeReasonOther, nil
	default:
		return "", bizerr.NewCode(customer.CodeWorkflowInvalid)
	}
}

func mapLegalChange(value *customer.LegalChangeRequest) *identitycap.EnterpriseLegalChange {
	if value == nil {
		return nil
	}
	fields := make([]identitycap.EnterpriseLegalChangeField, 0, len(value.Fields))
	for _, field := range value.Fields {
		fields = append(fields, identitycap.EnterpriseLegalChangeField(field))
	}
	return &identitycap.EnterpriseLegalChange{
		RequestNo: value.RequestNo, Status: identitycap.EnterpriseLegalChangeStatus(value.Status),
		Reason: identitycap.EnterpriseLegalChangeReason(value.Reason), Fields: fields,
		VerificationCaseNo:    value.VerificationCaseNo,
		CurrentProfileVersion: value.CurrentProfileVersion, SubmittedAt: value.SubmittedAt,
	}
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
