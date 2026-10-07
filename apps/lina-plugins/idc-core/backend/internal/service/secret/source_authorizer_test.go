package secret

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestSourceAuthorizerAllowsOnlyExactNotifyVerificationAccess(t *testing.T) {
	purpose, err := secretcap.NewPurpose("notify.verify")
	if err != nil {
		t.Fatal(err)
	}
	valid := AuthorizationRequest{
		Scope: Scope{
			TenantID:      0,
			OwnerPluginID: notifyOwnerPluginID,
			CallerScope:   notifyVerifyPrefix + "0123456789abcdef0123456789abcdef",
		},
		Action:  ActionAccess,
		Purpose: purpose,
	}
	authorizer := SourceAuthorizer{}
	if err = authorizer.Authorize(context.Background(), valid); err != nil {
		t.Fatalf("valid notify verification access rejected: %v", err)
	}

	for _, action := range []Action{ActionCreate, ActionRecoverCreate, ActionMetadata, ActionRotate} {
		request := valid
		request.Action, request.Purpose = action, ""
		if err = authorizer.Authorize(context.Background(), request); err != nil {
			t.Fatalf("valid notify verification lifecycle action %q rejected: %v", action, err)
		}
	}
	for _, action := range []Action{ActionUpdate, ActionRevoke} {
		request := valid
		request.Action, request.Purpose = action, ""
		if err = authorizer.Authorize(context.Background(), request); err == nil {
			t.Fatalf("forbidden notify verification lifecycle action %q accepted", action)
		}
	}

	tests := []struct {
		name   string
		mutate func(*AuthorizationRequest)
	}{
		{name: "other tenant", mutate: func(request *AuthorizationRequest) { request.Scope.TenantID = 1 }},
		{name: "other owner", mutate: func(request *AuthorizationRequest) { request.Scope.OwnerPluginID = coreOwnerPluginID }},
		{name: "channel scope", mutate: func(request *AuthorizationRequest) {
			request.Scope.CallerScope = notifyChannelPrefix + "email.c0123456789abcdef0123456789abcdef"
		}},
		{name: "uppercase instance", mutate: func(request *AuthorizationRequest) {
			request.Scope.CallerScope = notifyVerifyPrefix + "0123456789abcdef0123456789abcdeF"
		}},
		{name: "short instance", mutate: func(request *AuthorizationRequest) {
			request.Scope.CallerScope = notifyVerifyPrefix + "0123456789abcdef"
		}},
		{name: "extra suffix", mutate: func(request *AuthorizationRequest) {
			request.Scope.CallerScope += "0"
		}},
		{name: "other purpose", mutate: func(request *AuthorizationRequest) { request.Purpose = "notify.send" }},
		{name: "lifecycle with purpose", mutate: func(request *AuthorizationRequest) { request.Action = ActionCreate }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if authorizeErr := authorizer.Authorize(context.Background(), candidate); authorizeErr == nil {
				t.Fatal("unauthorized notify verification source request accepted")
			}
		})
	}
}

func TestNotifyVerificationGeneratedSecretPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureSecretIntegrationDB(t, ctx)
	service, err := newService(Config{}, fixedTestKeyProvider{}, SourceAuthorizer{}, time.Now, rand.Reader)
	if err != nil {
		t.Fatalf("create notify verification secret service: %v", err)
	}
	scope := Scope{
		TenantID:      0,
		OwnerPluginID: notifyOwnerPluginID,
		CallerScope:   notifyVerifyPrefix + "0123456789abcdef0123456789abcdef",
	}
	managerValue, err := service.Manager(scope)
	if err != nil {
		t.Fatalf("construct notify verification secret manager: %v", err)
	}
	manager, ok := managerValue.(secretcap.GeneratedManager)
	if !ok {
		t.Fatal("notify verification manager does not support generated secrets")
	}
	created, err := manager.CreateGenerated(ctx, "notify-verification-key", secretcap.GeneratedSecretPolicyURLSafe256)
	if err != nil {
		t.Fatalf("create notify verification key: %v", err)
	}
	replayed, err := manager.CreateGenerated(ctx, "notify-verification-key", secretcap.GeneratedSecretPolicyURLSafe256)
	if err != nil {
		t.Fatalf("replay notify verification key create: %v", err)
	}
	if created.Reused || !replayed.Reused || !created.Handle.Equal(replayed.Handle) {
		t.Fatalf("generated notify verification key did not replay exactly: created=%+v replayed=%+v", created, replayed)
	}

	purpose, err := secretcap.NewPurpose("notify.verify")
	if err != nil {
		t.Fatal(err)
	}
	request, err := secretcap.NewAccessRequest(created.Handle, purpose)
	if err != nil {
		t.Fatal(err)
	}
	accessor, err := service.Accessor(scope)
	if err != nil {
		t.Fatalf("construct notify verification accessor: %v", err)
	}
	var observed []byte
	if err = accessor.Access(ctx, request, func(value []byte) error {
		observed = append([]byte(nil), value...)
		return nil
	}); err != nil {
		t.Fatalf("access notify verification key: %v", err)
	}
	if len(observed) != 43 {
		t.Fatalf("unexpected generated notify verification key length: %d", len(observed))
	}
	defer zeroBytes(observed)

	crossScope := scope
	crossScope.CallerScope = notifyVerifyPrefix + "fedcba9876543210fedcba9876543210"
	crossAccessor, err := service.Accessor(crossScope)
	if err != nil {
		t.Fatalf("construct cross-scope notify verification accessor: %v", err)
	}
	if err = crossAccessor.Access(ctx, request, func(value []byte) error {
		if bytes.Equal(value, observed) {
			t.Fatal("cross-scope callback received notify verification key")
		}
		return nil
	}); err == nil {
		t.Fatal("cross-scope notify verification access succeeded")
	}
}

func TestSourceAuthorizerAllowsOnlyExactResourceChannelAccess(t *testing.T) {
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		t.Fatal(err)
	}
	valid := AuthorizationRequest{
		Scope: Scope{
			TenantID:      0,
			OwnerPluginID: resourceOwnerPluginID,
			CallerScope:   resourceChannelPrefix + "0123456789abcdef0123456789abcdef",
		},
		Action:  ActionAccess,
		Purpose: purpose,
	}
	authorizer := SourceAuthorizer{}
	if err = authorizer.Authorize(context.Background(), valid); err != nil {
		t.Fatalf("valid resource access rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*AuthorizationRequest)
	}{
		{name: "write with purpose", mutate: func(request *AuthorizationRequest) { request.Action = ActionCreate }},
		{name: "other tenant", mutate: func(request *AuthorizationRequest) { request.Scope.TenantID = 1 }},
		{name: "other owner", mutate: func(request *AuthorizationRequest) { request.Scope.OwnerPluginID = "idc-ai" }},
		{name: "broad scope", mutate: func(request *AuthorizationRequest) { request.Scope.CallerScope = "resource.channel.all" }},
		{name: "other purpose", mutate: func(request *AuthorizationRequest) { request.Purpose = "ai.configuration" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if authorizeErr := authorizer.Authorize(context.Background(), candidate); authorizeErr == nil {
				t.Fatal("unauthorized source request accepted")
			}
		})
	}
	write := valid
	write.Action = ActionCreate
	write.Purpose = ""
	if err = authorizer.Authorize(context.Background(), write); err != nil {
		t.Fatalf("valid resource lifecycle write rejected: %v", err)
	}
	write.Action = ActionRecoverCreate
	if err = authorizer.Authorize(context.Background(), write); err != nil {
		t.Fatalf("valid resource create recovery rejected: %v", err)
	}
}

func TestSourceAuthorizerAllowsOnlyExactPaymentChannelAccess(t *testing.T) {
	purpose, err := secretcap.NewPurpose(paymentChannelPurpose)
	if err != nil {
		t.Fatal(err)
	}
	valid := AuthorizationRequest{
		Scope: Scope{
			TenantID:      0,
			OwnerPluginID: paymentOwnerPluginID,
			CallerScope:   paymentChannelScope,
		},
		Action:  ActionAccess,
		Purpose: purpose,
	}
	authorizer := SourceAuthorizer{}
	if err = authorizer.Authorize(context.Background(), valid); err != nil {
		t.Fatalf("valid payment channel access rejected: %v", err)
	}
	for _, action := range []Action{ActionCreate, ActionRecoverCreate, ActionMetadata} {
		request := valid
		request.Action, request.Purpose = action, ""
		if err = authorizer.Authorize(context.Background(), request); err != nil {
			t.Fatalf("valid payment channel lifecycle action %q rejected: %v", action, err)
		}
	}
	for _, mutate := range []func(*AuthorizationRequest){
		func(request *AuthorizationRequest) { request.Scope.TenantID = 1 },
		func(request *AuthorizationRequest) { request.Scope.OwnerPluginID = notifyOwnerPluginID },
		func(request *AuthorizationRequest) { request.Scope.CallerScope += ".other" },
		func(request *AuthorizationRequest) { request.Purpose = "payment.channel-export" },
		func(request *AuthorizationRequest) { request.Action = ActionCreate },
		func(request *AuthorizationRequest) { request.Action, request.Purpose = ActionUpdate, "" },
		func(request *AuthorizationRequest) { request.Action, request.Purpose = ActionRevoke, "" },
		func(request *AuthorizationRequest) { request.Action, request.Purpose = ActionRotate, "" },
	} {
		request := valid
		mutate(&request)
		if err = authorizer.Authorize(context.Background(), request); err == nil {
			t.Fatal("unauthorized payment channel secret request accepted")
		}
	}
}

func TestSourceAuthorizerAllowsOnlyExactNotifyChannelAccess(t *testing.T) {
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		t.Fatal(err)
	}
	valid := AuthorizationRequest{
		Scope: Scope{
			TenantID:      0,
			OwnerPluginID: notifyOwnerPluginID,
			CallerScope:   notifyChannelPrefix + "email.c0123456789abcdef0123456789abcdef",
		},
		Action:  ActionAccess,
		Purpose: purpose,
	}
	authorizer := SourceAuthorizer{}
	if err = authorizer.Authorize(context.Background(), valid); err != nil {
		t.Fatalf("valid notify access rejected: %v", err)
	}
	signedWebhook := valid
	signedWebhook.Scope.CallerScope = notifyChannelPrefix + "signed-webhook.c0123456789abcdef0123456789abcdef"
	if err = authorizer.Authorize(context.Background(), signedWebhook); err != nil {
		t.Fatalf("valid signed webhook access rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*AuthorizationRequest)
	}{
		{name: "other tenant", mutate: func(request *AuthorizationRequest) { request.Scope.TenantID = 1 }},
		{name: "other owner", mutate: func(request *AuthorizationRequest) { request.Scope.OwnerPluginID = resourceOwnerPluginID }},
		{name: "broad channel", mutate: func(request *AuthorizationRequest) {
			request.Scope.CallerScope = notifyChannelPrefix + "all.c0123456789abcdef0123456789abcdef"
		}},
		{name: "missing instance", mutate: func(request *AuthorizationRequest) { request.Scope.CallerScope = notifyChannelPrefix + "email" }},
		{name: "uppercase instance", mutate: func(request *AuthorizationRequest) {
			request.Scope.CallerScope = notifyChannelPrefix + "email.c0123456789abcdef0123456789abcdeF"
		}},
		{name: "unlisted purpose", mutate: func(request *AuthorizationRequest) { request.Purpose = "notify.export" }},
		{name: "write with purpose", mutate: func(request *AuthorizationRequest) { request.Action = ActionCreate }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if authorizeErr := authorizer.Authorize(context.Background(), candidate); authorizeErr == nil {
				t.Fatal("unauthorized notify source request accepted")
			}
		})
	}
	write := valid
	write.Action = ActionCreate
	write.Purpose = ""
	if err = authorizer.Authorize(context.Background(), write); err != nil {
		t.Fatalf("valid notify lifecycle write rejected: %v", err)
	}
	write.Action = ActionRecoverCreate
	if err = authorizer.Authorize(context.Background(), write); err != nil {
		t.Fatalf("valid notify create recovery rejected: %v", err)
	}
}

func TestSourceAuthorizerAllowsOnlyExactCoreAnonymousSupportTokenScope(t *testing.T) {
	purpose, err := secretcap.NewPurpose("core.support-anonymous-access")
	if err != nil {
		t.Fatal(err)
	}
	valid := AuthorizationRequest{
		Scope:  Scope{TenantID: 0, OwnerPluginID: coreOwnerPluginID, CallerScope: coreSupportLinkScope},
		Action: ActionAccess, Purpose: purpose,
	}
	authorizer := SourceAuthorizer{}
	if err = authorizer.Authorize(context.Background(), valid); err != nil {
		t.Fatalf("valid core anonymous support access rejected: %v", err)
	}
	for _, mutate := range []func(*AuthorizationRequest){
		func(request *AuthorizationRequest) { request.Scope.TenantID = 1 },
		func(request *AuthorizationRequest) { request.Scope.OwnerPluginID = notifyOwnerPluginID },
		func(request *AuthorizationRequest) { request.Scope.CallerScope = "core.support" },
		func(request *AuthorizationRequest) { request.Purpose = "core.support-export" },
		func(request *AuthorizationRequest) { request.Action = ActionMetadata },
	} {
		candidate := valid
		mutate(&candidate)
		if authorizeErr := authorizer.Authorize(context.Background(), candidate); authorizeErr == nil {
			t.Fatal("unauthorized core anonymous support secret request accepted")
		}
	}
	write := valid
	write.Action, write.Purpose = ActionCreate, ""
	if err = authorizer.Authorize(context.Background(), write); err != nil {
		t.Fatalf("valid core anonymous support create rejected: %v", err)
	}
	write.Action = ActionRecoverCreate
	if err = authorizer.Authorize(context.Background(), write); err != nil {
		t.Fatalf("valid core anonymous support recovery rejected: %v", err)
	}
	write.Action = ActionUpdate
	if err = authorizer.Authorize(context.Background(), write); err == nil {
		t.Fatal("core anonymous support secret update accepted")
	}
}

func TestSourceAuthorizerAllowsOnlyInvoiceProfileBankLifecycle(t *testing.T) {
	authorizer := SourceAuthorizer{}
	valid := AuthorizationRequest{
		Scope: Scope{
			TenantID: 0, OwnerPluginID: coreOwnerPluginID,
			CallerScope: secretcap.CoreInvoiceProfileBankAccountScope,
		},
		Action: ActionCreate,
	}
	for _, action := range []Action{ActionCreate, ActionRecoverCreate, ActionMetadata} {
		candidate := valid
		candidate.Action = action
		if err := authorizer.Authorize(context.Background(), candidate); err != nil {
			t.Fatalf("valid invoice profile bank action %q rejected: %v", action, err)
		}
	}
	for _, mutate := range []func(*AuthorizationRequest){
		func(request *AuthorizationRequest) { request.Scope.TenantID = 1 },
		func(request *AuthorizationRequest) { request.Scope.OwnerPluginID = notifyOwnerPluginID },
		func(request *AuthorizationRequest) { request.Scope.CallerScope += ".other" },
		func(request *AuthorizationRequest) { request.Purpose = "core.invoice-profile" },
		func(request *AuthorizationRequest) { request.Action = ActionAccess },
		func(request *AuthorizationRequest) { request.Action = ActionUpdate },
		func(request *AuthorizationRequest) { request.Action = ActionRevoke },
		func(request *AuthorizationRequest) { request.Action = ActionRotate },
	} {
		candidate := valid
		mutate(&candidate)
		if err := authorizer.Authorize(context.Background(), candidate); err == nil {
			t.Fatal("unauthorized invoice profile bank secret request accepted")
		}
	}
}
