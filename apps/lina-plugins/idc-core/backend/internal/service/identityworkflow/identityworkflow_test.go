package identityworkflow

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type customerWorkflowStub struct {
	profile      *customer.Profile
	current      *customer.LegalChangeRequest
	requested    *customer.LegalChangeRequest
	requestInput customer.RequestOwnLegalChangeInput
}

func (s *customerWorkflowStub) CurrentProfile(context.Context) (*customer.Profile, error) {
	return s.profile, nil
}

func (s *customerWorkflowStub) CurrentOwnLegalChange(context.Context) (*customer.LegalChangeRequest, error) {
	return s.current, nil
}

func (s *customerWorkflowStub) RequestOwnLegalChange(
	_ context.Context,
	input customer.RequestOwnLegalChangeInput,
) (*customer.LegalChangeRequest, error) {
	s.requestInput = input
	return s.requested, nil
}

func TestEnterpriseAccountMapsCurrentProfileAndActiveChange(t *testing.T) {
	verifiedAt := time.Date(2026, time.August, 13, 9, 0, 0, 0, time.UTC)
	stub := &customerWorkflowStub{
		profile: &customer.Profile{
			Subject: customer.Subject{CustomerNo: "customer-001", Type: customer.TypeEnterprise},
			Enterprise: &customer.EnterpriseProfile{
				LegalName: "Example Ltd", RegistrationCountryCode: "CN",
				RegistrationIdentifierMasked: "91************0Y46",
				VerificationCaseNo:           "case-verified", VerificationStatus: customer.VerificationVerified,
				VerifiedAt: &verifiedAt, Version: 7,
			},
		},
		current: &customer.LegalChangeRequest{
			RequestNo: "LCR_0123456789abcdef0123456789abcdef",
			Status:    customer.LegalChangeStatusSubmitted, Reason: customer.LegalChangeReasonNameChange,
			Fields:             []customer.LegalChangeField{customer.LegalChangeFieldName},
			VerificationCaseNo: "case-verified", CurrentProfileVersion: 7, SubmittedAt: verifiedAt,
		},
	}
	service, err := New(stub)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.CurrentEnterpriseAccount(context.Background())
	if err != nil || state == nil || state.VerificationStatus != identitycap.StatusVerified ||
		state.RegistrationIdentifierMasked != "91************0Y46" || state.ProfileVersion != 7 ||
		state.ActiveLegalChange == nil || state.ActiveLegalChange.RequestNo != stub.current.RequestNo {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestEnterpriseLegalChangeDerivesCustomerAndMapsClosedValues(t *testing.T) {
	result := &customer.LegalChangeRequest{
		RequestNo: "LCR_0123456789abcdef0123456789abcdef",
		Status:    customer.LegalChangeStatusSubmitted, Reason: customer.LegalChangeReasonOther,
		Fields: []customer.LegalChangeField{
			customer.LegalChangeFieldName, customer.LegalChangeFieldIdentifier,
		},
	}
	stub := &customerWorkflowStub{
		profile: &customer.Profile{
			Subject:    customer.Subject{CustomerNo: "customer-001", Type: customer.TypeEnterprise},
			Enterprise: &customer.EnterpriseProfile{VerificationStatus: customer.VerificationVerified, Version: 4},
		},
		requested: result,
	}
	service, err := New(stub)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.RequestEnterpriseLegalChange(context.Background(), identitycap.EnterpriseLegalChangeInput{
		Fields: []identitycap.EnterpriseLegalChangeField{
			identitycap.EnterpriseLegalNameField, identitycap.EnterpriseLegalIdentifierField,
		},
		Reason:                 identitycap.EnterpriseOtherLegalChangeReason,
		ExpectedProfileVersion: 4, IdempotencyKey: "legal-change-idempotency-1",
	})
	if err != nil || created == nil || created.RequestNo != result.RequestNo ||
		stub.requestInput.CustomerNo != "customer-001" ||
		stub.requestInput.Reason != customer.LegalChangeReasonOther ||
		len(stub.requestInput.Fields) != 2 || stub.requestInput.ExpectedProfileVersion != 4 {
		t.Fatalf("created=%+v input=%+v err=%v", created, stub.requestInput, err)
	}
}

func TestEnterpriseWorkflowRejectsWrongSubjectAndUnknownField(t *testing.T) {
	service, err := New(&customerWorkflowStub{profile: &customer.Profile{
		Subject:    customer.Subject{CustomerNo: "customer-001", Type: customer.TypeIndividual},
		Individual: &customer.IndividualProfile{},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if state, stateErr := service.CurrentEnterpriseAccount(context.Background()); stateErr == nil || state != nil {
		t.Fatalf("individual state=%+v err=%v", state, stateErr)
	}

	service, err = New(&customerWorkflowStub{profile: &customer.Profile{
		Subject:    customer.Subject{CustomerNo: "customer-001", Type: customer.TypeEnterprise},
		Enterprise: &customer.EnterpriseProfile{VerificationStatus: customer.VerificationVerified},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if created, createErr := service.RequestEnterpriseLegalChange(
		context.Background(),
		identitycap.EnterpriseLegalChangeInput{Fields: []identitycap.EnterpriseLegalChangeField{"unknown"}},
	); createErr == nil || created != nil {
		t.Fatalf("unknown field result=%+v err=%v", created, createErr)
	}
	var typedNil *customerWorkflowStub
	if service, newErr := New(typedNil); newErr == nil || service != nil {
		t.Fatalf("typed nil service=%+v err=%v", service, newErr)
	}
}
