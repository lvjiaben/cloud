// These tests prove source-plugin product admission waits only for a known
// refusal before callback entry, never for an entered or opaque method result.
package providerexecution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/provideradmissioncap"
)

type productAdmissionHost struct {
	executionHost
	calls, refusals int
	enter           bool
	failure         error
}

func (h *productAdmissionHost) WithNewBusinessAdmission(ctx context.Context, _ pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	h.calls++
	if h.enter {
		return action(ctx, h.items[0])
	}
	if h.failure != nil {
		return h.failure
	}
	if h.calls <= h.refusals {
		return productAdmissionBusy()
	}
	return action(ctx, h.items[0])
}
func (h *productAdmissionHost) InvokeProviderMethod(context.Context, pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	h.calls++
	return nil, productAdmissionBusy()
}
func productAdmissionBusy() error {
	return bizerr.NewCode(bizerr.MustDefine("PLUGIN_LIFECYCLE_ALREADY_RUNNING", "private host explanation", gcode.CodeOperationFailed))
}

func TestProductAdmissionPreservesCallbackBoundary(t *testing.T) {
	for _, scenario := range []string{"before", "entered", "unknown"} {
		t.Run(scenario, func(t *testing.T) {
			evidence := &capmodel.BusinessExtensionProviderEvidence{PluginID: "connector", Point: "product-fulfillment", ContractVersion: "v1", ReleaseVersion: "1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{"provision"}}
			host := &productAdmissionHost{executionHost: executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{evidence}}, refusals: 2, enter: scenario == "entered"}
			if scenario == "unknown" {
				host.failure = errors.New("unconfirmed private response")
			}
			service, err := New(host)
			if err != nil {
				t.Fatal(err)
			}
			writes := 0
			err = service.WithNewBusinessAdmission(t.Context(), pluginhost.BusinessExtensionAdmissionInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, Method: "provision"}, func(context.Context, *capmodel.BusinessExtensionProviderEvidence) error {
				writes++
				if scenario == "entered" {
					return productAdmissionBusy()
				}
				return nil
			})
			switch scenario {
			case "before":
				if err != nil || host.calls != 3 || writes != 1 {
					t.Fatalf("refused admission calls=%d writes=%d err=%v", host.calls, writes, err)
				}
			case "entered":
				if !provideradmissioncap.IsBusy(err) || host.calls != 1 || writes != 1 {
					t.Fatal("entered business callback repeated")
				}
			case "unknown":
				if err == nil || host.calls != 1 || writes != 0 {
					t.Fatal("opaque failure repeated")
				}
			}
			host.calls = 0
			_, err = service.InvokeProviderMethod(t.Context(), pluginhost.BusinessExtensionMethodInvokeInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, Method: "provision", RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: "opaque-context"}})
			if !provideradmissioncap.IsBusy(err) || host.calls != 1 {
				t.Fatal("opaque runtime-context invocation automatically repeated")
			}
		})
	}
}
