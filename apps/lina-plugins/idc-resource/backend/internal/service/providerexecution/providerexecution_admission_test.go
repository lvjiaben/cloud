// These tests distinguish refused host admission from entered callbacks, and
// ensure bounded waiting cannot repeat a supplier call or binding transaction.

package providerexecution

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

func testAdmissionBusy() error {
	return bizerr.NewCode(bizerr.MustDefine("PLUGIN_LIFECYCLE_ALREADY_RUNNING", "private lifecycle explanation", gcode.CodeOperationFailed))
}

type contendedHost struct {
	executionHost
	calls    int
	refusals int
	entered  bool
}

func (h *contendedHost) InvokeProviderMethod(ctx context.Context, _ pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	h.calls++
	if h.entered {
		if _, err := h.outbound.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{}); err != nil {
			return nil, err
		}
	}
	if h.calls <= h.refusals {
		return nil, testAdmissionBusy()
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: []byte(`{}`)}, nil
}

func (h *contendedHost) WithNewBusinessAdmission(ctx context.Context, _ pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	h.calls++
	if h.entered {
		return action(ctx, h.items[0])
	}
	if h.calls <= h.refusals {
		return testAdmissionBusy()
	}
	return action(ctx, h.items[0])
}

func TestAdmissionWaitingRetainsConfidentialCallbackFence(t *testing.T) {
	for _, entered := range []bool{false, true} {
		t.Run(map[bool]string{false: "refused then admitted", true: "callback already entered"}[entered], func(t *testing.T) {
			evidence := &capmodel.BusinessExtensionProviderEvidence{PluginID: "connector", Point: "resource-provider", ContractVersion: "v1", ReleaseVersion: "v1", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{"execute"}}
			host := &contendedHost{executionHost: executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{evidence}}, refusals: 2, entered: entered}
			runtime, err := New(host)
			if err != nil {
				t.Fatal(err)
			}
			callbacks := 0
			err = runtime.WithConfidentialOutboundContext(t.Context(), pluginhost.ConfidentialOutboundContextInput{ProviderPluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, ProviderMethod: "execute", Handler: func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
				callbacks++
				return &pluginhost.ConfidentialOutboundResult{}, nil
			}}, func(ctx context.Context, runtimeContext *capmodel.BusinessExtensionRuntimeContext) error {
				_, invokeErr := runtime.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, Method: "execute", RuntimeContext: runtimeContext})
				return invokeErr
			})
			if entered {
				if !IsAdmissionBusy(err) || host.calls != 1 || callbacks != 1 {
					t.Fatal("entered callback was repeated")
				}
			} else if err != nil || host.calls != 3 || callbacks != 0 {
				t.Fatal("refused admission did not wait safely")
			}
		})
	}
}

func TestAdmissionWaitingNeverRepeatsEnteredBinding(t *testing.T) {
	for _, entered := range []bool{false, true} {
		evidence := &capmodel.BusinessExtensionProviderEvidence{PluginID: "connector", Point: "resource-provider", ContractVersion: "v1", ReleaseVersion: "v1", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{"execute"}}
		host := &contendedHost{executionHost: executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{evidence}}, refusals: 2, entered: entered}
		runtime, err := New(host)
		if err != nil {
			t.Fatal(err)
		}
		writes := 0
		err = runtime.WithNewBusinessAdmission(t.Context(), pluginhost.BusinessExtensionAdmissionInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion}, func(context.Context, *capmodel.BusinessExtensionProviderEvidence) error {
			writes++
			if entered {
				return testAdmissionBusy()
			}
			return nil
		})
		if writes != 1 || (entered && (!IsAdmissionBusy(err) || host.calls != 1)) || (!entered && (err != nil || host.calls != 3)) {
			t.Fatal("binding action admission boundary changed")
		}
	}
}

func TestAdmissionWaitStopsForOtherErrorsCancellationAndBound(t *testing.T) {
	// The public five-second policy permits at most forty waits and one initial call.
	const maximumCalls = 41
	for _, scenario := range []string{"unknown", "cancel", "bound"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := t.Context()
			if scenario == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 20*time.Millisecond)
				defer cancel()
			}
			calls := 0
			started := time.Now()
			err := waitForAdmission(ctx, func() (bool, error) {
				calls++
				if scenario == "unknown" {
					return false, errors.New("private transport error")
				}
				return false, testAdmissionBusy()
			})
			if err == nil || calls > maximumCalls || time.Since(started) > 7*time.Second {
				t.Fatal("unbounded admission waiting")
			}
			if scenario != "bound" && calls != 1 {
				t.Fatal("non-admission failure was repeated")
			}
			if scenario == "bound" && (calls != maximumCalls || !IsAdmissionBusy(err)) {
				t.Fatal("refusal category or bound lost")
			}
			if scenario == "cancel" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("cancellation lost")
			}
		})
	}
}
