package providerconfiguration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// configurationStatusRegistrar exposes current discovery only; historical lookup
// stays unavailable so a successful test proves management is release-independent.
type configurationStatusRegistrar struct {
	admissionRegistrar
	batchCalls int
}

func (registrar *configurationStatusRegistrar) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	registrar.batchCalls++
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: registrar.evidence}, nil
}

func currentConfigurationEvidence() *capmodel.BusinessExtensionProviderEvidence {
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "server-connector", ContractVersion: "v1", ReleaseVersion: "v2.0.0",
		InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "access_token", Type: capmodel.ConfigurationFieldSecret, Required: true},
		}},
		ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: resourcecap.DynamicProviderMetadataType, Version: "v1", Values: map[string]any{
			"providerId": "server-api",
			"authenticationProfiles": map[string]any{"profiles": map[string]any{"default": map[string]any{
				"strategy": "per_request_headers", "fields": map[string]any{"access-token": "access_token"},
			}}},
		}},
	}
}

func TestConfigurationUsesCurrentConnectorWithoutHistoricalRelease(t *testing.T) {
	registrar := &configurationStatusRegistrar{admissionRegistrar: admissionRegistrar{evidence: []*capmodel.BusinessExtensionProviderEvidence{currentConfigurationEvidence()}}}
	service := &service{extensions: registrar}
	channel := &Channel{ProviderPluginID: "server-connector", ProviderID: "server-api", ProviderContractVersion: "v1", ProviderReleaseVersion: "v1.0.0"}
	got, err := service.configurationProviderEvidence(t.Context(), channel)
	if err != nil || got.ReleaseVersion != "v2.0.0" {
		t.Fatalf("current configuration should not require saved release: got=%+v err=%v", got, err)
	}
	if _, err := service.validateUpdatedSettings(t.Context(), channel, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("saving settings must not require historical evidence: %v", err)
	}
	channel.ProviderID = "other-provider"
	if _, err := service.configurationProviderEvidence(t.Context(), channel); err == nil {
		t.Fatal("must reject another Provider identity")
	}
	channel.ProviderID = "server-api"
	channel.ProviderContractVersion = "v2"
	if _, err := service.configurationProviderEvidence(t.Context(), channel); err == nil {
		t.Fatal("must reject incompatible contracts")
	}
}

func TestProviderConfigurationPostgreSQLUsesCurrentSchemasInOneBatch(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	channels := make([]channelListRow, 100)
	for index := range channels {
		channels[index] = channelListRow{
			ChannelNo: fmt.Sprintf("CHN_%020d", index), ProviderRuntimeType: "dynamic",
			ProviderID: "server-api", ProviderPluginID: "server-connector", ResourceContractVersion: "v1",
			ResourceReleaseVersion: "v1.0.0", ResourceReleaseDigest: strings.Repeat("a", 64),
			ResourceDeclarationDigest: strings.Repeat("b", 64),
		}
	}
	channels[1].ProviderPluginID = "missing-connector"
	channels[3].ResourceContractVersion = "v2"
	evidence := currentConfigurationEvidence()
	registrar := &configurationStatusRegistrar{admissionRegistrar: admissionRegistrar{evidence: []*capmodel.BusinessExtensionProviderEvidence{evidence}}}
	service := &service{db: database, extensions: registrar}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel
 (channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code)
 VALUES (?,'server-api','server-connector','Lifecycle status','https://supplier.example','disabled')`, channels[0].ChannelNo); err != nil {
		t.Fatal(err)
	}
	boundAt := time.Unix(1780000000, 123456000).UTC()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel_secret_binding
 (binding_no,channel_no,binding_name,secret_ref,status_code,bound_at)
 VALUES ('BND_current_status',?,'access_token','sec_current_status','active',?)`, channels[0].ChannelNo, boundAt); err != nil {
		t.Fatal(err)
	}
	statuses, err := service.channelConfigurationStatuses(ctx, channels)
	if err != nil || len(statuses) != len(channels)-2 || registrar.batchCalls != 1 {
		t.Fatalf("expected one current discovery for 100 channels: statuses=%d calls=%d err=%v", len(statuses), registrar.batchCalls, err)
	}
	for index, channel := range channels {
		status, found := statuses[channel.ChannelNo]
		if index == 1 || index == 3 {
			if found {
				t.Fatalf("incompatible channel %d received configuration", index)
			}
			continue
		}
		if !found || status.InterfaceConfiguration != evidence.InterfaceConfiguration || len(status.SecretFields) != 1 || status.SecretFields[0].Ready != (index == 0) {
			t.Fatalf("channel %d missing current schema or invented secret: %+v", index, status)
		}
	}
	if got := statuses[channels[0].ChannelNo].SecretFields[0].UpdatedAt; !got.Equal(boundAt) {
		t.Fatalf("list must return the secret lifecycle time, not binding audit updated_at: got=%v want=%v", got, boundAt)
	}
	const channelNo = "CHN_missing000000000001"
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,provider_runtime_type,
     provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,
     display_name,endpoint,status_code)
VALUES (?, 'server-api','server-connector','dynamic','v1','v1.0.0',?,?,
        'Missing history','https://supplier.example','disabled')`, channelNo, strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.channel(ctx, channelNo); !bizerr.Is(err, CodeUnavailable) {
		// A missing interface binding has no compatible contract and remains unavailable.
		t.Fatalf("missing binding must remain unavailable: %v", err)
	}
}
