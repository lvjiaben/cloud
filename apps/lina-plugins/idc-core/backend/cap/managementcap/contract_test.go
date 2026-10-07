// This file verifies the narrow management authorization and sensitive-access contracts.

package managementcap

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type testAuthorizer struct{}

func (testAuthorizer) Authorize(context.Context, AuthorizationRequest) (int, error) { return 1, nil }

type testAudit struct{}

type testSensitiveAccess struct{}

func (testSensitiveAccess) IssueSensitiveAccess(
	context.Context,
	SensitiveAccessBinding,
) (*SensitiveAccessContext, error) {
	return &SensitiveAccessContext{Token: "proof", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (testSensitiveAccess) ConsumeSensitiveAccess(
	context.Context,
	string,
	SensitiveAccessBinding,
) (*SensitiveAccessProof, error) {
	return &SensitiveAccessProof{ActorUserID: 1, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (testAudit) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	return nil
}

func (testAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestAuthorizationRequestAndRuntimeValidation(t *testing.T) {
	t.Parallel()
	request := AuthorizationRequest{
		Permission: "idc:admin:catalog:product:list",
		TargetNo:   "inventory-snapshot-list",
	}
	if err := ValidateAuthorizationRequest(request); err != nil {
		t.Fatalf("ValidateAuthorizationRequest() error = %v", err)
	}
	request.Permission = "idc:admin:uncataloged"
	if err := ValidateAuthorizationRequest(request); err == nil {
		t.Fatal("ValidateAuthorizationRequest() accepted uncataloged permission")
	}
	if err := ValidateRuntime(Runtime{Authorizer: testAuthorizer{}, Audit: testAudit{}}); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
	if err := ValidateRuntime(Runtime{}); err == nil {
		t.Fatal("ValidateRuntime() accepted missing dependencies")
	}
	binding := SensitiveAccessBinding{TargetDigest: DigestSensitiveAccessTarget([]byte("target"))}
	if err := ValidateSensitiveAccessBinding(binding); err != nil {
		t.Fatalf("ValidateSensitiveAccessBinding() error = %v", err)
	}
	if err := ValidateSensitiveAccessBinding(SensitiveAccessBinding{TargetDigest: "BAD"}); err == nil {
		t.Fatal("ValidateSensitiveAccessBinding() accepted malformed digest")
	}
}
