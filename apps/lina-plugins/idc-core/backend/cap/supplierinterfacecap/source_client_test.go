package supplierinterfacecap

import (
	"context"
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/pluginhost"
)

type sourceInvokerStub struct {
	invocation   capregistry.Invocation
	secretCaller string
}

func (s *sourceInvokerStub) Invoke(_ context.Context, invocation capregistry.Invocation) (*capregistry.InvocationResult, error) {
	s.invocation = invocation
	text := "supplier-a"
	payload, _ := json.Marshal(&ConfigurationView{Values: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &text}}})
	return &capregistry.InvocationResult{Payload: payload}, nil
}

func (s *sourceInvokerStub) AccessSecret(ctx context.Context, _, _ string, consume func([]byte) error) error {
	s.secretCaller, _ = pluginhost.SourceBusinessExtensionCallerPluginID(ctx)
	plaintext := []byte("source-secret")
	defer clear(plaintext)
	return consume(plaintext)
}

func TestSourceClientBindsProviderIdentityToOwnerInvoker(t *testing.T) {
	t.Parallel()
	invoker := &sourceInvokerStub{}
	client, err := NewSourceClient(invoker, invoker)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.GetConfiguration(context.Background(), "src_0123456789abcdef0123456789abcdef"); err == nil {
		t.Fatal("ordinary context must not be allowed to choose a source identity")
	}
	if err = client.AccessSecret(context.Background(), "src_0123456789abcdef0123456789abcdef", "api_secret", func([]byte) error { return nil }); err == nil {
		t.Fatal("ordinary context must not access a source secret")
	}
	declarations := pluginhost.NewDeclarations("ssl-provider")
	if err = declarations.Providers().ProvideBusinessExtensionMethods(func(ctx context.Context, _ *capmodel.BusinessExtensionMethodInvocation) (*capmodel.BusinessExtensionMethodResult, error) {
		_, callErr := client.GetConfiguration(ctx, "src_0123456789abcdef0123456789abcdef")
		if callErr == nil {
			callErr = client.AccessSecret(ctx, "src_0123456789abcdef0123456789abcdef", "api_secret", func(value []byte) error {
				if string(value) != "source-secret" {
					t.Fatalf("unexpected callback plaintext")
				}
				return nil
			})
		}
		return &capmodel.BusinessExtensionMethodResult{Version: capmodel.BusinessExtensionInvocationProtocolV1}, callErr
	}); err != nil {
		t.Fatal(err)
	}
	definition := declarations.(pluginhost.SourcePluginDefinition)
	if _, err = definition.GetBusinessExtensionMethodHandler()(context.Background(), &capmodel.BusinessExtensionMethodInvocation{}); err != nil {
		t.Fatal(err)
	}
	if invoker.invocation.CallerPluginID != "ssl-provider" || invoker.invocation.ResourceRef != "src_0123456789abcdef0123456789abcdef" || invoker.invocation.Service != ServiceName || invoker.invocation.Method != MethodConfigurationGet {
		t.Fatalf("source invocation was not caller-bound: %#v", invoker.invocation)
	}
	if invoker.secretCaller != "ssl-provider" {
		t.Fatalf("secret callback was not host-bound: %q", invoker.secretCaller)
	}
}
