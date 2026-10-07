package composition

import (
	"regexp"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestBindChannelSecretsUsesValidStableScopeSegments(t *testing.T) {
	source := &channelSecretSourceStub{
		manager:  &verificationManagerStub{},
		accessor: verificationAccessorStub{},
	}
	bindings, err := bindChannelSecrets(source)
	if err != nil {
		t.Fatalf("bind channel secrets: %v", err)
	}
	if len(bindings) != 5 || len(source.scopes) != 10 {
		t.Fatalf("unexpected binding counts: bindings=%d scopes=%d", len(bindings), len(source.scopes))
	}
	valid := regexp.MustCompile(`^notify\.channel\.(email|sms|wechat-official|wechat-work|signed-webhook)\.c[0-9a-f]{32}$`)
	for _, scope := range source.scopes {
		if scope.TenantID != 0 || scope.OwnerPluginID != notifyOwnerPluginID || !valid.MatchString(scope.CallerScope) {
			t.Fatalf("channel secret scope escaped the contract: %+v", scope)
		}
	}
}

type channelSecretSourceStub struct {
	manager  secretcap.Manager
	accessor secretcap.Accessor
	scopes   []secretcap.Scope
}

func (s *channelSecretSourceStub) Manager(scope secretcap.Scope) (secretcap.Manager, error) {
	s.scopes = append(s.scopes, scope)
	return s.manager, nil
}

func (s *channelSecretSourceStub) Accessor(scope secretcap.Scope) (secretcap.Accessor, error) {
	s.scopes = append(s.scopes, scope)
	return s.accessor, nil
}
