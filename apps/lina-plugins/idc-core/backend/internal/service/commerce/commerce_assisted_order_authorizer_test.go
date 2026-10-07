package commerce

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestAssistedQuoteAuthorizerRequiresCreateAndCustomerQueryPermissions(t *testing.T) {
	current := riskReviewBizCtxStub{current: bizctxcap.CurrentContext{
		UserID: 81, TenantID: 0, DataScope: 2,
	}}
	staff := riskReviewPrincipalStub{audience: &principal.Audience{
		UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	for _, testCase := range []struct {
		name   string
		grants map[authz.PermissionKey]bool
		allow  bool
	}{
		{
			name: "both exact permissions",
			grants: map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionAssistedQuoteCreate):   true,
				authz.PermissionKey(permissionAssistedCustomerQuery): true,
			},
			allow: true,
		},
		{
			name: "missing customer query",
			grants: map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionAssistedQuoteCreate): true,
			},
		},
		{
			name: "customer query alone",
			grants: map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionAssistedCustomerQuery): true,
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			authorizer, err := NewAssistedQuoteAuthorizer(
				current, staff, riskReviewAuthzStub{grants: testCase.grants},
			)
			if err != nil {
				t.Fatalf("create assisted quote authorizer: %v", err)
			}
			actor, authorizeErr := authorizer.AuthorizeAssistedQuoteCreate(
				context.Background(), "CUS_ASSISTED_0001",
			)
			if testCase.allow {
				if authorizeErr != nil || actor != 81 {
					t.Fatalf("expected allow actor=%d err=%v", actor, authorizeErr)
				}
				return
			}
			if !bizerr.Is(authorizeErr, CodeAssistedQuoteNotAccessible) || actor != 0 {
				t.Fatalf("incomplete permission set allowed actor=%d err=%v", actor, authorizeErr)
			}
		})
	}
}

func TestAssistedQuoteAuthorizerKeepsWorkflowPermissionsIndependent(t *testing.T) {
	authorizer, err := NewAssistedQuoteAuthorizer(
		riskReviewBizCtxStub{current: bizctxcap.CurrentContext{
			UserID: 81, TenantID: 0, DataScope: 1,
		}},
		riskReviewPrincipalStub{audience: &principal.Audience{
			UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
		}},
		riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionAssistedQuoteApprove): true,
		}},
	)
	if err != nil {
		t.Fatalf("create assisted quote authorizer: %v", err)
	}
	requestNo := "AQR_0123456789abcdef0123456789abcdef"
	if actor, authorizeErr := authorizer.AuthorizeAssistedQuoteApprove(
		context.Background(), requestNo,
	); authorizeErr != nil || actor != 81 {
		t.Fatalf("approve permission actor=%d err=%v", actor, authorizeErr)
	}
	if actor, authorizeErr := authorizer.AuthorizeAssistedQuoteReject(
		context.Background(), requestNo,
	); !bizerr.Is(authorizeErr, CodeAssistedQuoteNotAccessible) || actor != 0 {
		t.Fatalf("approve grant leaked into rejection actor=%d err=%v", actor, authorizeErr)
	}
	if actor, authorizeErr := authorizer.AuthorizeAssistedQuoteCheckout(
		context.Background(), requestNo,
	); !bizerr.Is(authorizeErr, CodeAssistedQuoteNotAccessible) || actor != 0 {
		t.Fatalf("approve grant leaked into checkout actor=%d err=%v", actor, authorizeErr)
	}
}

func TestAssistedQuoteAuthorizerFailsClosedForUnsafeActors(t *testing.T) {
	staff := &principal.Audience{
		UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
	for _, testCase := range []struct {
		name     string
		current  bizctxcap.CurrentContext
		audience *principal.Audience
		allow    bool
	}{
		{name: "direct staff", current: bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2}, audience: staff, allow: true},
		{name: "customer principal", current: bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2}, audience: &principal.Audience{UserID: 81, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "CUS_81"}},
		{name: "impersonated", current: bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2, IsImpersonation: true}, audience: staff},
		{name: "delegated", current: bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2, ActingUserID: 82}, audience: staff},
		{name: "unsupported scope", current: bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 3}, audience: staff},
		{name: "foreign tenant", current: bizctxcap.CurrentContext{UserID: 81, TenantID: 3, DataScope: 2}, audience: staff},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			authorizer, err := NewAssistedQuoteAuthorizer(
				riskReviewBizCtxStub{current: testCase.current},
				riskReviewPrincipalStub{audience: testCase.audience},
				riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{
					authz.PermissionKey(permissionAssistedQuoteList): true,
				}},
			)
			if err != nil {
				t.Fatalf("create assisted quote authorizer: %v", err)
			}
			actor, authorizeErr := authorizer.AuthorizeAssistedQuoteList(context.Background())
			if testCase.allow {
				if authorizeErr != nil || actor != 81 {
					t.Fatalf("expected allow actor=%d err=%v", actor, authorizeErr)
				}
				return
			}
			if !bizerr.Is(authorizeErr, CodeAssistedQuoteNotAccessible) || actor != 0 {
				t.Fatalf("unsafe actor allowed actor=%d err=%v", actor, authorizeErr)
			}
		})
	}
}
