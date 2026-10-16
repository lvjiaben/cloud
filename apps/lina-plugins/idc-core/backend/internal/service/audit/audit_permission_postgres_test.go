// This file verifies the exact cataloged secret-management permission survives
// real audit sealing, replay and integrity-checked reads without widening the
// credential-bearing text exception to arbitrary permission strings.
package audit

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestAuditSecretPermissionSealedReplayPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	const permission = "idc:admin:resource:channel-secret:manage"
	service, err := newService(contextActorResolver{}, staticAuthorizationService{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permission): true, authz.PermissionKey(permissionAuditRead): true}}, testAuditConfig(), time.Now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ctx = withAuditIntegrationActor(ctx, TrustedActor{Type: ActorUser, UserID: 602, AudienceCode: principal.AudienceStaff, DataScope: hostDataScopeTenant})
	input := auditIntegrationTransition("resource.channel-secret.create", "secret-permission-one")
	input.RequiredPermission = permission
	input.CustomerNo = ""
	input.OperationCode = "resource.channel-secret.create.complete"
	input.TargetType = "resource_channel_secret"
	input.TargetNo = "channel-integration"
	input.BeforeSnapshot = map[string]any{}
	input.AfterSnapshot = map[string]any{"channelNo": input.TargetNo, "bindingName": "appsecret", "statusCode": "active", "version": 1}
	created := appendAuditIntegrationTransition(t, ctx, service, input)
	replayed := appendAuditIntegrationTransition(t, ctx, service, input)
	if replayed.AuditNo != created.AuditNo {
		t.Fatal("secret permission broke sealed replay")
	}
	read, err := service.Search(ctx, SearchInput{TargetType: input.TargetType, TargetNo: input.TargetNo, Limit: 10})
	if err != nil || read == nil || len(read.Events) != 1 || read.Events[0].AuditNo != created.AuditNo {
		t.Fatalf("sealed secret event read: %+v %v", read, err)
	}
	assertAuditIntegrationCounts(t, ctx, 1, 1)
}
