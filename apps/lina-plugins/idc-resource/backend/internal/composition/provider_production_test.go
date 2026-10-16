// This file exercises the production environment factory so confidential Basic
// authentication cannot lose the channel-scoped accessor during startup wiring.
package composition

import (
	"context"
	"encoding/json"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

func TestProductionProviderEnvironmentBindsBasicAuthenticationAccessor(t *testing.T) {
	// Construction needs a DAO database object but performs no database I/O.
	original := gdb.GetAllConfig()
	if err := gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: {{Link: "pgsql:test:test@tcp(127.0.0.1:1)/production_factory"}}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := gdb.SetConfig(original); err != nil {
			t.Error(err)
		}
	})
	var (
		accessor      = confidentialHTTPAccessor{"secret_app_identity": []byte("app_test"), "secret_app_secret": []byte("test-secret")}
		source        = &captureSource{value: accessor, manager: inertManager{}}
		configuration = channelconfig.Configuration{Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_production", Endpoint: "https://example.com",
			Settings: json.RawMessage(`{"__host":{"policy":{"mode":"per_request_basic","mappings":[{"role":"username","slot":"app_id"},{"role":"password","slot":"app_secret"}]}}}`),
		}}
	)
	user, err := secretcap.NewHandle("secret_app_identity")
	if err != nil {
		t.Fatal(err)
	}
	password, err := secretcap.NewHandle("secret_app_secret")
	if err != nil {
		t.Fatal(err)
	}
	configuration.Config.SecretBindings = []resourcecap.SecretBinding{{Name: "app_id", Handle: user}, {Name: "app_secret", Handle: password}}
	environment, cleanup, err := productionProviderEnvironment(context.Background(), configuration, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if environment.Secrets == nil || environment.AuthenticationSessions == nil || len(source.accessorScopes) != 1 || len(source.managerScopes) != 1 || source.accessorScopes[0] != source.managerScopes[0] {
		t.Fatal("production environment omitted channel-scoped authentication dependencies")
	}
	handler := &confidentialHTTPHandler{configuration: configuration, environment: environment}
	settings, err := decodeChannelAuthenticationSettings(configuration.Config.Settings)
	if err != nil {
		t.Fatal(err)
	}
	request := resourcecap.ConfidentialHTTPRequest{}
	called := false
	err = handler.withBasicAuthentication(context.Background(), &request, settings, func() error {
		called = true
		if request.Headers["Authorization"] != "Basic YXBwX3Rlc3Q6dGVzdC1zZWNyZXQ=" {
			t.Fatal("production credentials were not injected")
		}
		return nil
	})
	if err != nil || !called || len(request.Headers) != 0 {
		t.Fatalf("production Basic boundary failed: called=%v err=%v", called, err)
	}
}
