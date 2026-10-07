package providerconfiguration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
)

type configurationStatusRegistrar struct {
	admissionRegistrar
	inputs     []plugincap.BusinessExtensionProviderExactInput
	batchCalls int
}

func (registrar *configurationStatusRegistrar) GetProvidersExact(_ context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	registrar.batchCalls++
	registrar.inputs = inputs
	return registrar.evidence, nil
}

func TestProviderConfigurationPostgreSQLIsolatesMissingHistoricalSchemas(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	channels := make([]channelListRow, 100)
	evidence := make([]*capmodel.BusinessExtensionProviderEvidence, len(channels))
	for index := range channels {
		channels[index] = channelListRow{
			ChannelNo: fmt.Sprintf("CHN_%020d", index), ProviderRuntimeType: "dynamic",
			ProviderPluginID: "server-connector", ResourceContractVersion: "v1",
			ResourceReleaseVersion: "v1.0.0", ResourceReleaseDigest: strings.Repeat("a", 64),
			ResourceDeclarationDigest: strings.Repeat("b", 64),
		}
		evidence[index] = &capmodel.BusinessExtensionProviderEvidence{
			InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{
				Fields: []*capmodel.ConfigurationFieldDeclaration{
					{Key: "access_token", Type: capmodel.ConfigurationFieldSecret, Required: true},
				},
			},
		}
	}
	// Discovery preserves order and returns a nil slot for an absent release.
	evidence[1] = nil
	evidence[3].InterfaceConfiguration = nil
	registrar := &configurationStatusRegistrar{admissionRegistrar: admissionRegistrar{evidence: evidence}}
	service := &service{db: database, extensions: registrar}
	statuses, err := service.channelConfigurationStatuses(ctx, channels)
	if err != nil || len(statuses) != len(channels)-2 {
		t.Fatalf("missing historical rows discarded healthy configurations: count=%d err=%v", len(statuses), err)
	}
	if registrar.batchCalls != 1 || len(registrar.inputs) != len(channels) {
		t.Fatalf("expected one exact-evidence batch for all rows, calls=%d inputs=%d", registrar.batchCalls, len(registrar.inputs))
	}
	for index, channel := range channels {
		status, found := statuses[channel.ChannelNo]
		if index == 1 || index == 3 {
			if found {
				t.Fatalf("missing row %d received another release's schema", index)
			}
			continue
		}
		if !found || status.InterfaceConfiguration != evidence[index].InterfaceConfiguration || len(status.SecretFields) != 1 || status.SecretFields[0].Ready {
			t.Fatalf("healthy row %d has incorrect schema or invented secret readiness: %+v", index, status)
		}
		if registrar.inputs[index].ReleaseVersion != channel.ResourceReleaseVersion || registrar.inputs[index].DeclarationDigest != channel.ResourceDeclarationDigest {
			t.Fatalf("row %d did not preserve frozen release identity", index)
		}
	}
	const missingChannelNo = "CHN_missing000000000001"
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,provider_runtime_type,
     provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,
     display_name,endpoint,status_code)
VALUES (?, 'server-api','server-connector','dynamic','v1','v1.0.0',?,?,
        'Missing history','https://supplier.example','disabled')`, missingChannelNo, strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	registrar.evidence = []*capmodel.BusinessExtensionProviderEvidence{nil}
	if _, err := service.channel(ctx, missingChannelNo); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("mutation lookup must reject a missing exact schema: %v", err)
	}
}
