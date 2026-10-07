package identityprojection

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/identitycap"
)

func TestEnterpriseProjectionRejectsDecisionMetadataOnPendingState(t *testing.T) {
	service := &serviceImpl{}
	_, err := service.ProjectEnterprise(context.Background(), identitycap.EnterpriseProjectionInput{
		CustomerNo: "cust_enterprise_0001", CaseNo: "case_enterprise_0001",
		Status: identitycap.StatusPending, DecisionReasonCode: "unexpected_reason",
	})
	if err == nil {
		t.Fatal("pending enterprise projection accepted terminal decision metadata")
	}
}

func TestEnterpriseProjectionRejectsInvalidExpiryAndClearsRawBuffers(t *testing.T) {
	service := &serviceImpl{}
	verifiedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	expiresAt := verifiedAt
	legalName := []byte("Example Enterprise")
	registration := []byte("91110000MA0012345X")
	_, err := service.ProjectEnterprise(context.Background(), identitycap.EnterpriseProjectionInput{
		CustomerNo: "cust_enterprise_0001", CaseNo: "case_enterprise_0001",
		Status: identitycap.StatusVerified, ReviewActorUserID: 9001,
		DecisionReasonCode: "manual_review_approved", LegalName: legalName,
		RegistrationCountryCode: "CN", RegistrationIdentifier: registration,
		MaskedRegistrationIdentifier: "9111**********345X", VerifiedAt: &verifiedAt,
		ExpiresAt: &expiresAt,
	})
	if err == nil {
		t.Fatal("enterprise projection accepted a non-increasing expiry")
	}
	assertIdentityBytesCleared(t, legalName)
	assertIdentityBytesCleared(t, registration)
}
