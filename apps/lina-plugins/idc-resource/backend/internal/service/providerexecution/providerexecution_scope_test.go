// These tests count discovery independently of actual host guards. A locked
// current declaration is reusable only by its registrar during the callback;
// retained contexts, partial batches and another identity must rediscover.
package providerexecution

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
)

type scopedAdmissionHost struct {
	executionHost
	guards int
}

func (h *scopedAdmissionHost) WithNewBusinessAdmission(ctx context.Context, input pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	h.guards++
	h.admission = input
	return action(ctx, h.items[0])
}
func scopeTestEvidence(plugin, point, contract string) *capmodel.BusinessExtensionProviderEvidence {
	return &capmodel.BusinessExtensionProviderEvidence{PluginID: plugin, Point: point, ContractVersion: contract, ReleaseVersion: "v2.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{"provision"}}
}
func scopeTestInput(evidence *capmodel.BusinessExtensionProviderEvidence) plugincap.BusinessExtensionProviderExactInput {
	return plugincap.BusinessExtensionProviderExactInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("c", 64), DeclarationDigest: strings.Repeat("d", 64)}
}

func TestAdmissionCurrentEvidenceAvoidsDiscoveryWithoutSkippingHostCalls(t *testing.T) {
	current := scopeTestEvidence("connector", "product-fulfillment", "v1")
	other := scopeTestEvidence("another", "product-fulfillment", "v1")
	otherPoint := scopeTestEvidence("connector", "resource-provider", "v1")
	otherContract := scopeTestEvidence("connector", "product-fulfillment", "v2")
	host := &scopedAdmissionHost{executionHost: executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{current, other, otherPoint, otherContract}}}
	runtime, err := New(host)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(host)
	if err != nil {
		t.Fatal(err)
	}
	input := scopeTestInput(current)
	var retained context.Context
	err = runtime.WithNewBusinessAdmission(t.Context(), pluginhost.BusinessExtensionAdmissionInput{PluginID: input.PluginID, Point: input.Point, ContractVersion: input.ContractVersion, Method: "provision", ReleaseVersion: input.ReleaseVersion, ReleaseDigest: input.ReleaseDigest, DeclarationDigest: input.DeclarationDigest}, func(ctx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
		retained = ctx
		if host.reads != 0 || host.guards != 1 || evidence != current || host.admission.ReleaseVersion != "" || host.admission.ReleaseDigest != "" || host.admission.DeclarationDigest != "" {
			t.Fatal("admission performed discovery before the authoritative host guard")
		}
		selected, readErr := runtime.GetProviderExact(ctx, input)
		if readErr != nil || selected.ReleaseDigest != current.ReleaseDigest {
			t.Fatalf("current scoped proof missing: %v", readErr)
		}
		selected.AuthorizedMethods[0] = "forbidden"
		selected, readErr = runtime.GetProviderExact(ctx, input)
		if readErr != nil || selected.AuthorizedMethods[0] != "provision" {
			t.Fatal("caller altered scope method authority")
		}
		batch, readErr := runtime.GetProvidersExact(ctx, []plugincap.BusinessExtensionProviderExactInput{input, input})
		if readErr != nil || len(batch) != 2 || host.reads != 0 {
			t.Fatal("fully admitted batch rediscovered provider")
		}
		err := runtime.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{ProviderPluginID: input.PluginID, Point: input.Point, ContractVersion: input.ContractVersion, ProviderMethod: "provision"}, func(callCtx context.Context, runtimeContext *capmodel.BusinessExtensionRuntimeContext) error {
			_, invokeErr := runtime.InvokeProviderMethod(callCtx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: input.PluginID, Point: input.Point, ContractVersion: input.ContractVersion, Method: "provision", RuntimeContext: runtimeContext})
			return invokeErr
		})
		if err != nil || host.reads != 0 || host.outbound.ReleaseDigest != current.ReleaseDigest || host.invocation.ReleaseDigest != current.ReleaseDigest {
			t.Fatalf("scope bypassed a host call or repeated discovery: reads=%d err=%v", host.reads, err)
		}
		if _, err = runtime.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: input.PluginID, Point: input.Point, ContractVersion: input.ContractVersion, Method: "forbidden"}); err == nil {
			t.Fatal("scoped proof extended method authority")
		}
		before := host.reads
		if _, err = second.GetProviderExact(ctx, input); err != nil || host.reads != before+1 {
			t.Fatal("another registrar borrowed the scope")
		}
		for _, otherEvidence := range []*capmodel.BusinessExtensionProviderEvidence{other, otherPoint, otherContract} {
			before = host.reads
			if _, err = runtime.GetProviderExact(ctx, scopeTestInput(otherEvidence)); err != nil || host.reads != before+1 {
				t.Fatal("another identity borrowed the scope")
			}
		}
		before = host.reads
		if _, err = runtime.GetProvidersExact(ctx, []plugincap.BusinessExtensionProviderExactInput{input, scopeTestInput(other)}); err != nil || host.reads != before+1 {
			t.Fatal("partial batch did not use one fresh collection")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := host.reads
	if _, err = runtime.GetProviderExact(retained, input); err != nil || host.reads != before+1 {
		t.Fatal("retained context reused expired admission evidence")
	}
	host.items = nil
	if _, err = runtime.GetProviderExact(retained, input); err == nil {
		t.Fatal("expired scope concealed disabled provider")
	}
}

func TestAdmissionRejectsInvalidHostProofBeforeOwnerCallback(t *testing.T) {
	for _, scenario := range []string{"nil", "plugin", "point", "contract", "method", "release"} {
		t.Run(scenario, func(t *testing.T) {
			evidence := scopeTestEvidence("connector", "product-fulfillment", "v1")
			host := &scopedAdmissionHost{executionHost: executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{evidence}}}
			input := pluginhost.BusinessExtensionAdmissionInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, Method: "provision"}
			switch scenario {
			case "nil":
				host.items[0] = nil
			case "plugin":
				evidence.PluginID = "other"
			case "point":
				evidence.Point = "resource-provider"
			case "contract":
				evidence.ContractVersion = "v2"
			case "method":
				evidence.AuthorizedMethods = []string{"query"}
			case "release":
				evidence.ReleaseDigest = ""
			}
			runtime, err := New(host)
			if err != nil {
				t.Fatal(err)
			}
			callbacks := 0
			err = runtime.WithNewBusinessAdmission(t.Context(), input, func(context.Context, *capmodel.BusinessExtensionProviderEvidence) error { callbacks++; return nil })
			if err == nil || callbacks != 0 || host.guards != 1 || host.reads != 0 {
				t.Fatal("invalid guard evidence entered the owner or caused rediscovery/retry")
			}
		})
	}
}

func TestAdmissionScopeExpiresOnErrorAndPanic(t *testing.T) {
	for _, panicExit := range []bool{false, true} {
		evidence := scopeTestEvidence("connector", "product-fulfillment", "v1")
		host := &scopedAdmissionHost{executionHost: executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{evidence}}}
		runtime, err := New(host)
		if err != nil {
			t.Fatal(err)
		}
		var retained context.Context
		func() {
			if panicExit {
				defer func() {
					if recover() == nil {
						t.Error("fixture panic was swallowed")
					}
				}()
			}
			err = runtime.WithNewBusinessAdmission(t.Context(), pluginhost.BusinessExtensionAdmissionInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, Method: "provision"}, func(ctx context.Context, _ *capmodel.BusinessExtensionProviderEvidence) error {
				retained = ctx
				if panicExit {
					panic("fixture exit")
				}
				return errors.New("unknown callback outcome")
			})
		}()
		if !panicExit && err == nil {
			t.Fatal("owner error was swallowed")
		}
		if host.guards != 1 {
			t.Fatal("entered callback repeated")
		}
		if _, err = runtime.GetProviderExact(retained, scopeTestInput(evidence)); err != nil || host.reads != 1 {
			t.Fatal("failed callback left scope active")
		}
	}
}

func TestAdmissionScopeSupportsIndependentConcurrentReads(t *testing.T) {
	evidence := scopeTestEvidence("connector", "product-fulfillment", "v1")
	host := &scopedAdmissionHost{executionHost: executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{evidence}}}
	runtime, err := New(host)
	if err != nil {
		t.Fatal(err)
	}
	err = runtime.WithNewBusinessAdmission(t.Context(), pluginhost.BusinessExtensionAdmissionInput{PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion, Method: "provision"}, func(ctx context.Context, _ *capmodel.BusinessExtensionProviderEvidence) error {
		var workers sync.WaitGroup
		failures := make(chan error, 16)
		for range 16 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for range 100 {
					selected, e := runtime.GetProviderExact(ctx, scopeTestInput(evidence))
					if e != nil {
						failures <- e
						return
					}
					if selected.AuthorizedMethods[0] != "provision" {
						failures <- errors.New("scope shared a mutable method slice")
						return
					}
					selected.AuthorizedMethods[0] = "caller-only"
				}
			}()
		}
		workers.Wait()
		close(failures)
		for failure := range failures {
			t.Error(failure)
		}
		return nil
	})
	if err != nil || host.reads != 0 || host.guards != 1 {
		t.Fatalf("parallel locked reads rediscovered or lost authority: %v", err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	before := host.guards
	err = runtime.WithNewBusinessAdmission(canceled, pluginhost.BusinessExtensionAdmissionInput{}, func(context.Context, *capmodel.BusinessExtensionProviderEvidence) error {
		t.Fatal("canceled callback entered")
		return nil
	})
	if !errors.Is(err, context.Canceled) || host.guards != before {
		t.Fatal("cancellation did not stop before host admission")
	}
}
