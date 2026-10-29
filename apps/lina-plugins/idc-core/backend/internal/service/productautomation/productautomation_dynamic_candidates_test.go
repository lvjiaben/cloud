package productautomation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestDynamicCandidatesBatchCacheAndExactEvidence(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	extensions := &automationTestExtensions{}
	extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		var request configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &request); err != nil {
			t.Fatalf("decode invocation: %v", err)
		}
		if input.Purpose != capmodel.BusinessExtensionInvocationPurposeNewBusiness || len(request.Requests) != 2 {
			t.Fatalf("invocation was not one bounded new-business batch: %#v", input)
		}
		if input.RuntimeContext == nil || len(input.RuntimeContext.ContextID) != len("src_")+32 || input.RuntimeContext.ContextID[:4] != "src_" {
			t.Fatalf("invocation omitted its opaque supplier runtime context: %#v", input.RuntimeContext)
		}
		items := make([]configurationcontract.DynamicCandidateItem, 0, len(request.Requests))
		for _, query := range request.Requests {
			items = append(items, configurationcontract.DynamicCandidateItem{FieldKey: query.FieldKey, Candidates: []*capmodel.ConfigurationCandidate{
				{Value: query.FieldKey + "-a", Label: &capmodel.LocalizedText{ZhCN: "选项", EnUS: "Option"}},
			}})
		}
		payload, _ := json.Marshal(configurationcontract.DynamicCandidateResponse{SchemaVersion: "v1", Items: items, Relationships: []resourceinterfaceprojectioncap.ChoiceRelationship{{ParentFieldKey: "region", ChildFieldKey: "model", Links: []resourceinterfaceprojectioncap.ChoiceLink{{ParentValue: "region-a", ChildValue: "model-a"}}}}})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: payload}, nil
	}
	cache := newDynamicCandidateCache()
	cache.now = func() time.Time { return now }
	service := &serviceImpl{extensions: extensions, dynamicCandidates: cache, runtimeContexts: newSupplierRuntimeContextStore(rand.Reader)}
	exact := dynamicCandidateEvidence("1.0.0")
	schema := exact.CustomerConfiguration
	var relationships []resourceinterfaceprojectioncap.ChoiceRelationship
	scope := dynamicCandidateScope{interfaceInstanceNo: "pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", configurationRevision: 1, candidateRelationships: &relationships}

	resolved, err := service.resolveDynamicCandidates(context.Background(), exact, schema, nil, configurationSurfaceCustomer, scope)
	if err != nil || len(resolved) != 2 || extensions.invokeCalls != 1 {
		t.Fatalf("first batch resolve failed: resolved=%#v calls=%d err=%v", resolved, extensions.invokeCalls, err)
	}
	if len(relationships) != 1 {
		t.Fatal("generic provider relationship was lost")
	}
	relationships[0].Links[0].ParentValue = "caller-mutation"
	relationships = nil
	if _, err = service.resolveDynamicCandidates(context.Background(), exact, schema, nil, configurationSurfaceCustomer, scope); err != nil || extensions.invokeCalls != 1 {
		t.Fatalf("same exact input missed cache: calls=%d err=%v", extensions.invokeCalls, err)
	}
	if len(relationships) != 1 || relationships[0].Links[0].ParentValue != "region-a" {
		t.Fatal("cache hit lost or mutated eligibility facts")
	}
	selected := []ConfigurationValue{{FieldKey: "region", OptionTokens: []string{"region-a"}}}
	if _, err = service.resolveDynamicCandidates(context.Background(), exact, schema, selected, configurationSurfaceCustomer, scope); err != nil || extensions.invokeCalls != 2 {
		t.Fatalf("changed current selection reused cache: calls=%d err=%v", extensions.invokeCalls, err)
	}
	otherRelease := dynamicCandidateEvidence("1.0.1")
	if _, err = service.resolveDynamicCandidates(context.Background(), otherRelease, otherRelease.CustomerConfiguration, nil, configurationSurfaceCustomer, scope); err != nil || extensions.invokeCalls != 3 {
		t.Fatalf("changed exact release reused cache: calls=%d err=%v", extensions.invokeCalls, err)
	}
	otherInterface := dynamicCandidateScope{interfaceInstanceNo: "pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", configurationRevision: 1}
	if _, err = service.resolveDynamicCandidates(context.Background(), exact, schema, nil, configurationSurfaceCustomer, otherInterface); err != nil || extensions.invokeCalls != 4 {
		t.Fatalf("supplier A/B interfaces shared a candidate cache entry: calls=%d err=%v", extensions.invokeCalls, err)
	}
	otherRevision := dynamicCandidateScope{interfaceInstanceNo: scope.interfaceInstanceNo, configurationRevision: 2}
	if _, err = service.resolveDynamicCandidates(context.Background(), exact, schema, nil, configurationSurfaceCustomer, otherRevision); err != nil || extensions.invokeCalls != 5 {
		t.Fatalf("supplier configuration revisions shared a candidate cache entry: calls=%d err=%v", extensions.invokeCalls, err)
	}
	now = now.Add(dynamicCandidateCacheTTL)
	if _, err = service.resolveDynamicCandidates(context.Background(), exact, schema, nil, configurationSurfaceCustomer, scope); err != nil || extensions.invokeCalls != 6 {
		t.Fatalf("expired cache was not rebuilt: calls=%d err=%v", extensions.invokeCalls, err)
	}
}

func TestDynamicCandidatesUseCrossFieldCurrentSelections(t *testing.T) {
	exact := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "ssl-provider", ReleaseVersion: "1.0.0", Point: capmodel.BusinessExtensionProductFulfillment,
		ContractVersion: "v1", ReleaseDigest: "sha256:release", DeclarationDigest: "sha256:declaration",
		AuthorizedMethods: []string{"candidate.region"},
		CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "country", Type: capmodel.ConfigurationFieldSingleSelect, Label: dynamicCandidateLabel("国家", "Country"), StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "cn", Label: dynamicCandidateLabel("中国", "China")}}},
			{Key: "region", Type: capmodel.ConfigurationFieldSingleSelect, Label: dynamicCandidateLabel("地区", "Region"), DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.region"}},
		}},
	}
	extensions := &automationTestExtensions{}
	extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		var request configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &request); err != nil {
			return nil, err
		}
		if len(request.CurrentSelections) != 1 || request.CurrentSelections[0].FieldKey != "country" ||
			len(request.CurrentSelections[0].Values) != 1 || request.CurrentSelections[0].Values[0] != "cn" {
			t.Fatalf("region resolver did not receive the country selection: %#v", request.CurrentSelections)
		}
		payload, err := json.Marshal(configurationcontract.DynamicCandidateResponse{
			SchemaVersion: "v1", Items: []configurationcontract.DynamicCandidateItem{{
				FieldKey: "region", Candidates: []*capmodel.ConfigurationCandidate{{Value: "cn-zj", Label: dynamicCandidateLabel("浙江", "Zhejiang")}},
			}},
		})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: payload}, err
	}
	service := &serviceImpl{extensions: extensions, dynamicCandidates: newDynamicCandidateCache(), runtimeContexts: newSupplierRuntimeContextStore(rand.Reader)}
	values := []ConfigurationValue{{FieldKey: "country", OptionTokens: []string{"cn"}}}
	resolved, err := service.resolveDynamicCandidates(context.Background(), exact, exact.CustomerConfiguration, values, configurationSurfaceCustomer,
		dynamicCandidateScope{interfaceInstanceNo: "pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", configurationRevision: 1})
	if err != nil || len(resolved["region"]) != 1 || resolved["region"][0].Value != "cn-zj" {
		t.Fatalf("cross-field dynamic candidates failed: resolved=%#v err=%v", resolved, err)
	}
}

func TestDynamicCandidateCacheIsBoundedAndReturnsCopies(t *testing.T) {
	cache := newDynamicCandidateCache()
	for index := 0; index <= dynamicCandidateCacheEntries; index++ {
		key := time.Unix(int64(index), 0).String()
		cache.put(key, dynamicCandidateProjection{candidates: configurationcontract.ResolvedCandidates{"field": {{Value: key, Label: &capmodel.LocalizedText{ZhCN: "值", EnUS: "Value"}}}}})
	}
	if cache.order.Len() != dynamicCandidateCacheEntries || len(cache.entries) != dynamicCandidateCacheEntries {
		t.Fatalf("cache exceeded bound: order=%d entries=%d", cache.order.Len(), len(cache.entries))
	}
	value, ok := cache.get(time.Unix(int64(dynamicCandidateCacheEntries), 0).String())
	if !ok {
		t.Fatal("newest cache entry missing")
	}
	value.candidates["field"][0].Value = "mutated"
	again, ok := cache.get(time.Unix(int64(dynamicCandidateCacheEntries), 0).String())
	if !ok || again.candidates["field"][0].Value == "mutated" {
		t.Fatal("caller mutated cached candidate state")
	}
}

func TestDynamicCandidateCacheReleasesWaitersAfterLoaderPanic(t *testing.T) {
	cache := newDynamicCandidateCache()
	started := make(chan struct{})
	release := make(chan struct{})
	results := make(chan error, 2)
	loader := func() (dynamicCandidateProjection, error) {
		close(started)
		<-release
		panic("sensitive candidate panic")
	}
	go func() {
		_, err := cache.load(context.Background(), "panic-key", loader)
		results <- err
	}()
	<-started
	waiterStarted := make(chan struct{})
	go func() {
		close(waiterStarted)
		_, err := cache.load(context.Background(), "panic-key", func() (dynamicCandidateProjection, error) {
			return dynamicCandidateProjection{}, errors.New("concurrent waiter unexpectedly became the loader")
		})
		results <- err
	}()
	<-waiterStarted
	deadline := time.Now().Add(time.Second)
	for {
		cache.mu.Lock()
		current := cache.inflight["panic-key"]
		joined := current != nil && current.waiters == 1
		cache.mu.Unlock()
		if joined {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("concurrent candidate cache waiter did not join inflight call")
		}
		runtime.Gosched()
	}
	close(release)
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err == nil {
				t.Fatal("panic was not converted into a closed failure")
			}
		case <-time.After(time.Second):
			t.Fatal("candidate cache waiter remained blocked after panic")
		}
	}
	value, err := cache.load(context.Background(), "panic-key", func() (dynamicCandidateProjection, error) {
		return dynamicCandidateProjection{candidates: configurationcontract.ResolvedCandidates{"field": {{Value: "reloaded"}}}}, nil
	})
	if err != nil || len(value.candidates["field"]) != 1 || value.candidates["field"][0].Value != "reloaded" {
		t.Fatalf("candidate cache did not reload after panic: value=%#v err=%v", value, err)
	}
}

func TestDynamicCandidatesPrefetchRuntimeConfigurationOnceAcrossMethods(t *testing.T) {
	exact := dynamicCandidateEvidence("1.0.0")
	exact.AuthorizedMethods = []string{"candidate.model", "candidate.region"}
	exact.CustomerConfiguration.Fields[0].DynamicCandidates.Method = "candidate.region"
	exact.CustomerConfiguration.Fields[1].DynamicCandidates.Method = "candidate.model"
	loaderCalls := 0
	extensions := &automationTestExtensions{}
	extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		var request configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &request); err != nil {
			return nil, err
		}
		items := make([]configurationcontract.DynamicCandidateItem, 0, len(request.Requests))
		for _, query := range request.Requests {
			items = append(items, configurationcontract.DynamicCandidateItem{FieldKey: query.FieldKey, Candidates: []*capmodel.ConfigurationCandidate{{Value: query.FieldKey, Label: dynamicCandidateLabel("选项", "Option")}}})
		}
		payload, err := json.Marshal(configurationcontract.DynamicCandidateResponse{SchemaVersion: "v1", Items: items})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: payload}, err
	}
	service := &serviceImpl{
		extensions: extensions, dynamicCandidates: newDynamicCandidateCache(), runtimeContexts: newSupplierRuntimeContextStore(rand.Reader),
		runtimeConfigurationLoader: func(context.Context, *capmodel.BusinessExtensionProviderEvidence, dynamicCandidateScope) (*supplierinterfacecap.ConfigurationView, error) {
			loaderCalls++
			return &supplierinterfacecap.ConfigurationView{Values: []supplierinterfacecap.ConfigurationValue{}}, nil
		},
	}
	_, err := service.resolveDynamicCandidates(context.Background(), exact, exact.CustomerConfiguration, nil, configurationSurfaceCustomer,
		dynamicCandidateScope{interfaceInstanceNo: "pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", configurationRevision: 1})
	if err != nil || loaderCalls != 1 || extensions.invokeCalls != 2 {
		t.Fatalf("multi-method prefetch: loader=%d provider=%d err=%v", loaderCalls, extensions.invokeCalls, err)
	}
}

func TestDynamicCandidateConcurrentCacheMissInvokesProviderOnce(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	extensions := &automationTestExtensions{}
	extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		startOnce.Do(func() { close(started) })
		<-release
		var request configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &request); err != nil {
			return nil, err
		}
		items := make([]configurationcontract.DynamicCandidateItem, 0, len(request.Requests))
		for _, query := range request.Requests {
			items = append(items, configurationcontract.DynamicCandidateItem{
				FieldKey: query.FieldKey,
				Candidates: []*capmodel.ConfigurationCandidate{{
					Value: query.FieldKey + "-a", Label: dynamicCandidateLabel("选项", "Option"),
				}},
			})
		}
		payload, err := json.Marshal(configurationcontract.DynamicCandidateResponse{SchemaVersion: "v1", Items: items})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: payload}, err
	}
	service := &serviceImpl{extensions: extensions, dynamicCandidates: newDynamicCandidateCache(), runtimeContexts: newSupplierRuntimeContextStore(rand.Reader)}
	exact := dynamicCandidateEvidence("1.0.0")
	const callers = 16
	scope := dynamicCandidateScope{interfaceInstanceNo: "pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", configurationRevision: 1}
	ready := make(chan struct{})
	errors := make(chan error, callers)
	var waiting sync.WaitGroup
	waiting.Add(callers)
	for index := 0; index < callers; index++ {
		go func() {
			waiting.Done()
			<-ready
			_, err := service.resolveDynamicCandidates(
				context.Background(), exact, exact.CustomerConfiguration, nil, configurationSurfaceCustomer, scope,
			)
			errors <- err
		}()
	}
	waiting.Wait()
	close(ready)
	<-started
	close(release)
	for index := 0; index < callers; index++ {
		if err := <-errors; err != nil {
			t.Fatalf("concurrent candidate resolution failed: %v", err)
		}
	}
	if extensions.invokeCalls != 1 {
		t.Fatalf("concurrent cache miss invoked provider %d times, want 1", extensions.invokeCalls)
	}
}

func dynamicCandidateEvidence(release string) *capmodel.BusinessExtensionProviderEvidence {
	label := &capmodel.LocalizedText{ZhCN: "字段", EnUS: "Field"}
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "phone-provider", ReleaseVersion: release, Point: capmodel.BusinessExtensionProductFulfillment,
		ContractVersion: "v1", ReleaseDigest: "sha256:release", DeclarationDigest: "sha256:declaration",
		AuthorizedMethods: []string{"candidate.options"},
		CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "region", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.options"}},
			{Key: "model", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.options"}},
		}},
	}
}

func dynamicCandidateLabel(zhCN, enUS string) *capmodel.LocalizedText {
	return &capmodel.LocalizedText{ZhCN: zhCN, EnUS: enUS}
}

func TestCandidateFailureDiagnosticNeverCopiesUnknownText(t *testing.T) {
	if got := safeCandidateFailureReason(errors.New("credential-and-address-must-not-be-logged")); got != "unclassified" {
		t.Fatal(got)
	}
	if got := safeCandidateFailureReason(errors.New("resolve: resource interface candidate result is invalid")); got != "candidate result is invalid" {
		t.Fatal(got)
	}
}
