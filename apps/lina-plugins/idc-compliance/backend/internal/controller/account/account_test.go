// This file verifies account controller dependency and request-mapping boundaries.

package account

import (
	"context"
	"errors"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
	"lina-plugin-idc-core/backend/cap/identitycap"
)


type accountPrivacyStub struct {
	privacygovernance.Service
	overview       *privacygovernance.CustomerPrivacyOverview
	withdrawInput  identitycap.PrivacyConsentWithdrawalInput
	withdrawResult *identitycap.PrivacyConsentDecision
}

func (s *accountPrivacyStub) CurrentCustomerPrivacy(context.Context) (*privacygovernance.CustomerPrivacyOverview, error) {
	return s.overview, nil
}

func (s *accountPrivacyStub) WithdrawCurrentCustomerConsent(_ context.Context, input identitycap.PrivacyConsentWithdrawalInput) (*identitycap.PrivacyConsentDecision, error) {
	s.withdrawInput = input
	return s.withdrawResult, nil
}









type identityServiceStub struct {
	consentInput identitycap.PersonalConsentInput
	consent      *identitycap.PersonalConsent
}

func (*identityServiceStub) CurrentPersonal(context.Context) (*identitycap.PersonalSubject, error) {
	return &identitycap.PersonalSubject{CustomerNo: "cust_test", ActorUserID: 8101}, nil
}

func (s *identityServiceStub) RecordPersonalConsent(
	_ context.Context,
	input identitycap.PersonalConsentInput,
) (*identitycap.PersonalConsent, error) {
	s.consentInput = input
	return s.consent, nil
}

func (*identityServiceStub) PreparePersonal(
	context.Context,
	string,
) (*identitycap.PersonalSubject, error) {
	return nil, errors.New("not used")
}

func (*identityServiceStub) ProjectPersonal(
	context.Context,
	identitycap.PersonalProjectionInput,
) (*identitycap.PersonalProjection, error) {
	return nil, errors.New("not used")
}

func (*identityServiceStub) CurrentEnterprise(context.Context) (*identitycap.EnterpriseSubject, error) {
	return &identitycap.EnterpriseSubject{CustomerNo: "cust_test", ActorUserID: 8101}, nil
}

func (*identityServiceStub) RecordEnterpriseConsent(
	context.Context,
	identitycap.EnterpriseConsentInput,
) (*identitycap.EnterpriseConsent, error) {
	return nil, errors.New("not used")
}

func (*identityServiceStub) PrepareEnterprise(
	context.Context,
	string,
	string,
) (*identitycap.EnterprisePreparation, error) {
	return nil, errors.New("not used")
}

func (*identityServiceStub) ProjectEnterprise(
	context.Context,
	identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, error) {
	return nil, errors.New("not used")
}

type enterpriseVerificationStub struct {
	current *enterpriseverification.CustomerCase
}

func (s *enterpriseVerificationStub) Current(context.Context) (*enterpriseverification.CustomerCase, error) {
	return s.current, nil
}

func (*enterpriseVerificationStub) CreateDraft(
	context.Context,
	enterpriseverification.CreateDraftInput,
) (*enterpriseverification.Draft, error) {
	return nil, errors.New("not used")
}

func (*enterpriseVerificationStub) ReserveMaterial(
	context.Context,
	enterpriseverification.ReserveMaterialInput,
) (*enterpriseverification.MaterialReservation, error) {
	return nil, errors.New("not used")
}

func (*enterpriseVerificationStub) UploadMaterial(
	context.Context,
	enterpriseverification.UploadMaterialInput,
) (*enterpriseverification.Material, error) {
	return nil, errors.New("not used")
}

func (*enterpriseVerificationStub) Submit(
	context.Context,
	enterpriseverification.SubmitInput,
) (*enterpriseverification.Submission, error) {
	return nil, errors.New("not used")
}

type enterpriseAccountStub struct {
	state       *identitycap.EnterpriseAccountState
	change      *identitycap.EnterpriseLegalChange
	changeInput identitycap.EnterpriseLegalChangeInput
}

func (s *enterpriseAccountStub) CurrentEnterpriseAccount(context.Context) (*identitycap.EnterpriseAccountState, error) {
	return s.state, nil
}

func (s *enterpriseAccountStub) RequestEnterpriseLegalChange(
	_ context.Context,
	input identitycap.EnterpriseLegalChangeInput,
) (*identitycap.EnterpriseLegalChange, error) {
	s.changeInput = input
	return s.change, nil
}

type verificationServiceStub struct {
	current      *personalverification.Verification
	started      *personalverification.Verification
	queried      *personalverification.Verification
	startInput   personalverification.StartInput
	queryAttempt string
	currentCalls int
	queryCalls   int
}

func (s *verificationServiceStub) Current(context.Context) (*personalverification.Verification, error) {
	s.currentCalls++
	return s.current, nil
}

func (s *verificationServiceStub) Start(
	_ context.Context,
	input personalverification.StartInput,
) (*personalverification.Verification, error) {
	s.startInput = input
	s.startInput.CertName = append([]byte(nil), input.CertName...)
	s.startInput.CertNo = append([]byte(nil), input.CertNo...)
	return s.started, nil
}

func (s *verificationServiceStub) QueryCurrent(
	_ context.Context,
	attemptNo string,
) (*personalverification.Verification, error) {
	s.queryCalls++
	s.queryAttempt = attemptNo
	return s.queried, nil
}

func (*verificationServiceStub) Recover(
	context.Context,
	int,
) (*personalverification.RecoveryResult, error) {
	return nil, errors.New("not used")
}

func TestAccountControllerSeparatesConsentAndTrustedInitiation(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 123000000, time.UTC)
	identities := &identityServiceStub{consent: &identitycap.PersonalConsent{
		ConsentNo:     "CNS_0123456789abcdef0123456789abcdef",
		PolicyVersion: identitycap.PersonalConsentPolicyVersion, OccurredAt: now,
	}}
	verification := &verificationServiceStub{started: &personalverification.Verification{
		CaseNo:          "case_0123456789abcdef0123456789abcdef",
		AttemptNo:       "pva_0123456789abcdef0123456789abcdef",
		Status:          personalverification.StatusVerifying,
		ContinuationURL: "https://cloudauth.aliyuncs.com/verify/safe",
	}}
	controller, err := NewV1(
		identities, verification, &enterpriseVerificationStub{}, &enterpriseAccountStub{},
		"https://portal.example.test/account/identity?verification_return=1",
	)
	if err != nil {
		t.Fatal(err)
	}
	consent, err := controller.PersonalVerificationConsentCreate(
		context.Background(), &v1.PersonalVerificationConsentCreateReq{
			Accepted: true, PolicyVersion: identitycap.PersonalConsentPolicyVersion,
			Locale: "zh-CN", IdempotencyKey: "personal-consent-01J5M6S2R9Z8X7W6",
		},
	)
	if err != nil || consent == nil || consent.ConsentNo != identities.consent.ConsentNo ||
		consent.OccurredAt != now.UnixMilli() {
		t.Fatalf("consent=%+v err=%v", consent, err)
	}
	if identities.consentInput.IdempotencyKey != "personal-consent-01J5M6S2R9Z8X7W6" {
		t.Fatalf("consent input=%+v", identities.consentInput)
	}
	request := &v1.PersonalVerificationCreateReq{
		ConsentNo: consent.ConsentNo, CertType: "IDENTITY_CARD",
		CertName: "张三", CertNo: "110101200002290019",
	}
	started, err := controller.PersonalVerificationCreate(context.Background(), request)
	if err != nil || started == nil || started.Item == nil ||
		started.Item.ContinuationURL != verification.started.ContinuationURL {
		t.Fatalf("started=%+v err=%v", started, err)
	}
	if request.CertName != "" || request.CertNo != "" ||
		verification.startInput.ReturnURL != "https://portal.example.test/account/identity?verification_return=1" ||
		string(verification.startInput.CertName) != "张三" ||
		string(verification.startInput.CertNo) != "110101200002290019" {
		t.Fatalf("request=%+v start=%+v", request, verification.startInput)
	}
}

func TestAccountControllerLocalReadAndAuthoritativeWriteAreDistinct(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC)
	identities := &identityServiceStub{}
	verification := &verificationServiceStub{
		current: &personalverification.Verification{Status: personalverification.StatusVerifying},
		queried: &personalverification.Verification{
			AttemptNo: "pva_0123456789abcdef0123456789abcdef",
			Status:    personalverification.StatusVerified, VerifiedAt: &verifiedAt,
		},
	}
	controller, err := NewV1(
		identities, verification, &enterpriseVerificationStub{}, &enterpriseAccountStub{},
		"https://portal.example.test/account/identity?verification_return=1",
	)
	if err != nil {
		t.Fatal(err)
	}
	local, err := controller.PersonalVerificationGet(
		context.Background(), &v1.PersonalVerificationGetReq{},
	)
	if err != nil || local == nil || local.Item.StatusCode != "verifying" ||
		verification.currentCalls != 1 || verification.queryCalls != 0 {
		t.Fatalf("local=%+v calls=%d/%d err=%v", local, verification.currentCalls, verification.queryCalls, err)
	}
	refreshed, err := controller.PersonalVerificationAuthoritativeStateUpdate(
		context.Background(), &v1.PersonalVerificationAuthoritativeStateUpdateReq{
			AttemptNo: "pva_0123456789abcdef0123456789abcdef",
		},
	)
	if err != nil || refreshed == nil || refreshed.Item.StatusCode != "verified" ||
		refreshed.Item.VerifiedAt != verifiedAt.UnixMilli() || verification.queryCalls != 1 ||
		verification.queryAttempt != "pva_0123456789abcdef0123456789abcdef" {
		t.Fatalf("refreshed=%+v query=%q calls=%d err=%v", refreshed, verification.queryAttempt, verification.queryCalls, err)
	}
}

func TestAccountControllerRejectsMissingConsentAndUntrustedReturnURL(t *testing.T) {
	identities := &identityServiceStub{}
	verification := &verificationServiceStub{}
	if controller, err := NewV1(identities, verification, &enterpriseVerificationStub{}, &enterpriseAccountStub{}, "https://evil.example.test/return"); err == nil || controller != nil {
		t.Fatalf("controller=%v err=%v", controller, err)
	}
	if controller, err := NewV1(identities, verification, &enterpriseVerificationStub{}, &enterpriseAccountStub{}, "https://:443/account/identity?verification_return=1"); err == nil || controller != nil {
		t.Fatalf("empty-host controller=%v err=%v", controller, err)
	}
	controller, err := NewV1(
		identities, verification, &enterpriseVerificationStub{}, &enterpriseAccountStub{},
		"https://portal.example.test/account/identity?verification_return=1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if loopbackController, loopbackErr := NewV1(
		identities, verification, &enterpriseVerificationStub{}, &enterpriseAccountStub{},
		"http://127.0.0.1:6032/account/identity?verification_return=1",
	); loopbackErr != nil || loopbackController == nil {
		t.Fatalf("explicit local development return URL rejected: controller=%v err=%v", loopbackController, loopbackErr)
	}
	if result, createErr := controller.PersonalVerificationConsentCreate(
		context.Background(), &v1.PersonalVerificationConsentCreateReq{Accepted: false},
	); result != nil || !bizerr.Is(createErr, personalverification.CodeInvalid) {
		t.Fatalf("result=%+v err=%v", result, createErr)
	}
}

func TestAccountControllerMapsEnterpriseStateAndLegalChangeWithoutCustomerInput(t *testing.T) {
	now := time.Date(2026, time.August, 13, 10, 0, 0, 0, time.UTC)
	enterprise := &enterpriseVerificationStub{current: &enterpriseverification.CustomerCase{
		CaseNo: "ent_0123456789abcdef0123456789abcdef", StatusCode: "pending",
		SubmittedAt: &now, UpdatedAt: &now, Version: 2,
	}}
	account := &enterpriseAccountStub{
		state: &identitycap.EnterpriseAccountState{
			VerificationCaseNo: "ent_verified", VerificationStatus: identitycap.StatusVerified,
			LegalName: "Example Ltd", RegistrationIdentifierMasked: "91************0Y46",
			VerifiedAt: &now, ProfileVersion: 7,
		},
		change: &identitycap.EnterpriseLegalChange{
			RequestNo: "LCR_0123456789abcdef0123456789abcdef",
			Status:    identitycap.EnterpriseLegalChangeSubmitted,
			Reason:    identitycap.EnterpriseRegistrationChangeReason,
			Fields: []identitycap.EnterpriseLegalChangeField{
				identitycap.EnterpriseLegalNameField, identitycap.EnterpriseLegalIdentifierField,
			},
			VerificationCaseNo: "ent_verified", CurrentProfileVersion: 7, SubmittedAt: now,
		},
	}
	controller, err := NewV1(
		&identityServiceStub{}, &verificationServiceStub{}, enterprise, account,
		"https://portal.example.test/account/identity?verification_return=1",
	)
	if err != nil {
		t.Fatal(err)
	}
	state, err := controller.EnterpriseVerificationGet(context.Background(), &v1.EnterpriseVerificationGetReq{})
	if err != nil || state == nil || state.Item == nil || state.Item.Case == nil ||
		state.Item.Case.StatusCode != "pending" || state.Item.Profile == nil ||
		state.Item.Profile.ProfileVersion != 7 || state.Item.PolicyVersion != "1" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	created, err := controller.EnterpriseLegalChangeCreate(
		context.Background(),
		&v1.EnterpriseLegalChangeCreateReq{
			FieldCodes: []string{"legal_name", "legal_identifier"},
			ReasonCode: "enterprise_registration_change", ExpectedProfileVersion: 7,
			IdempotencyKey: "enterprise-legal-change-idempotency-1",
		},
	)
	if err != nil || created == nil || created.Item == nil ||
		created.Item.RequestNo != account.change.RequestNo || len(account.changeInput.Fields) != 2 ||
		account.changeInput.ExpectedProfileVersion != 7 {
		t.Fatalf("created=%+v input=%+v err=%v", created, account.changeInput, err)
	}
}

func TestAccountControllerMapsCurrentCustomerPrivacyWithoutCustomerIdentifier(t *testing.T) {
	now := time.Date(2026, time.August, 18, 11, 0, 0, 0, time.UTC)
	privacy := &accountPrivacyStub{
		overview: &privacygovernance.CustomerPrivacyOverview{
			Consents: []identitycap.PrivacyConsentDecision{{
				ConsentNo:   "PVC_0123456789abcdef0123456789abcdef",
				ConsentType: identitycap.PrivacyConsentSensitivePersonal,
				ActionCode:  "granted", OccurredAt: now,
			}},

		},
		withdrawResult: &identitycap.PrivacyConsentDecision{
			ConsentNo:   "PVC_1123456789abcdef0123456789abcdef",
			ConsentType: identitycap.PrivacyConsentSensitivePersonal,
			ActionCode:  "withdrawn", OccurredAt: now,
		},

	}
	controller := &ControllerV1{privacy: privacy}
	overview, err := controller.CustomerPrivacyGet(t.Context(), &v1.CustomerPrivacyGetReq{})
	if err != nil || len(overview.Consents) != 1 ||
		overview.Consents[0].ConsentTypeCode != "sensitive_personal_information" {
		t.Fatalf("CustomerPrivacyGet() = %#v error=%v", overview, err)
	}
	withdrawn, err := controller.CustomerPrivacyConsentWithdrawCreate(t.Context(), &v1.CustomerPrivacyConsentWithdrawCreateReq{
		ConsentTypeCode: "sensitive_personal_information", IdempotencyKey: "privacy-withdraw-controller-0001",
	})
	if err != nil || withdrawn.Consent.ActionCode != "withdrawn" || privacy.withdrawInput.ConsentType != identitycap.PrivacyConsentSensitivePersonal {
		t.Fatalf("CustomerPrivacyConsentWithdrawCreate() = %#v input=%#v error=%v", withdrawn, privacy.withdrawInput, err)
	}
}
