package productautomation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type runtimeFileReaderStub struct {
	scope purchaseconfigurationfile.ConsumedFileScope
	data  []byte
}

func (s *runtimeFileReaderStub) ListConsumedField(_ context.Context, scope purchaseconfigurationfile.ConsumedFileScope, fieldKey string) ([]purchaseconfigurationfile.ConsumedFile, error) {
	if scope != s.scope || fieldKey != "certificate" {
		return nil, errors.New("CFU_SECRET private/internal/object https://storage.invalid")
	}
	return []purchaseconfigurationfile.ConsumedFile{{Sequence: 1, FileName: "request.csr", MediaType: "text/plain", SizeBytes: int64(len(s.data)), SHA256Hex: strings.Repeat("a", 64), ObjectRef: "private/internal/object"}}, nil
}

func (s *runtimeFileReaderStub) OpenConsumedFieldFile(ctx context.Context, scope purchaseconfigurationfile.ConsumedFileScope, fieldKey string, sequence int) (*purchaseconfigurationfile.OpenConsumedFile, error) {
	files, err := s.ListConsumedField(ctx, scope, fieldKey)
	if err != nil || sequence != 1 {
		return nil, io.ErrUnexpectedEOF
	}
	return &purchaseconfigurationfile.OpenConsumedFile{File: files[0], Body: io.NopCloser(bytes.NewReader(s.data))}, nil
}

func TestRuntimeFilesUseExactInvocationScopeForDynamicAndSourceProviders(t *testing.T) {
	t.Parallel()
	scope := purchaseconfigurationfile.ConsumedFileScope{IntentNo: "PFI_TEST", OrderNo: "ORD_TEST", OrderItemNo: "ORI_TEST", ConfigurationSnapshotNo: "SCS_TEST",
		PluginID: "ssl-provider", ReleaseVersion: "1.0.0", ContractVersion: "v1", ReleaseDigest: strings.Repeat("b", 64), DeclarationDigest: strings.Repeat("c", 64),
		Method: "certificate.provision", InterfaceInstanceNo: "PAI_TEST", InterfaceConfigurationRevision: 1}
	service := &serviceImpl{runtimeContexts: newSupplierRuntimeContextStore(rand.Reader), runtimeFiles: newSupplierRuntimeFileStore(), consumedFiles: &runtimeFileReaderStub{scope: scope, data: []byte("abcdef")}}
	invoker := NewRuntimeCapabilityInvoker()
	invoker.service = service

	issue := func() supplierRuntimeContext {
		item, err := service.runtimeContexts.issue(&capmodel.BusinessExtensionProviderEvidence{PluginID: scope.PluginID, ReleaseVersion: scope.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: scope.ContractVersion, ReleaseDigest: scope.ReleaseDigest, DeclarationDigest: scope.DeclarationDigest, AuthorizedMethods: []string{scope.Method}}, dynamicCandidateScope{interfaceInstanceNo: scope.InterfaceInstanceNo, configurationRevision: scope.InterfaceConfigurationRevision}, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, scope.Method, nil, ExistingBusinessRuntimeContextInput{IntentNo: scope.IntentNo, OrderNo: scope.OrderNo, OrderItemNo: scope.OrderItemNo, ConfigurationSnapshotNo: scope.ConfigurationSnapshotNo})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}

	dynamic := issue()
	listPayload, _ := json.Marshal(supplierinterfacecap.ConfigurationFileListRequest{FieldKey: "certificate"})
	listed, err := invoker.Invoke(context.Background(), capregistry.Invocation{CallerPluginID: "ssl-provider", OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodFilesList, ResourceRef: dynamic.id, Payload: listPayload})
	if err != nil || bytes.Contains(listed.Payload, []byte("private/internal")) || bytes.Contains(listed.Payload, []byte("CFU_")) {
		t.Fatalf("safe dynamic list failed or leaked: %s %v", listed.Payload, err)
	}
	request := supplierinterfacecap.ConfigurationFileReadRequest{FieldKey: "certificate", Sequence: 1, MaximumBytes: 4}
	requestPayload, _ := json.Marshal(request)
	first, err := invoker.Invoke(context.Background(), capregistry.Invocation{CallerPluginID: "ssl-provider", OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodFileRead, ResourceRef: dynamic.id, Payload: requestPayload})
	if err != nil {
		t.Fatal(err)
	}
	var firstChunk supplierinterfacecap.ConfigurationFileChunk
	_ = json.Unmarshal(first.Payload, &firstChunk)
	if string(firstChunk.Data) != "abcd" || firstChunk.EOF {
		t.Fatalf("first chunk: %#v", firstChunk)
	}
	if _, err = invoker.Invoke(context.Background(), capregistry.Invocation{CallerPluginID: "phone-provider", OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodFilesList, ResourceRef: dynamic.id, Payload: listPayload}); err == nil || strings.Contains(err.Error(), "private/internal") {
		t.Fatalf("cross-plugin access was not safely rejected: %v", err)
	}
	wrongIntent, issueErr := service.runtimeContexts.issue(&capmodel.BusinessExtensionProviderEvidence{PluginID: scope.PluginID, ReleaseVersion: scope.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: scope.ContractVersion, ReleaseDigest: scope.ReleaseDigest, DeclarationDigest: scope.DeclarationDigest, AuthorizedMethods: []string{scope.Method}}, dynamicCandidateScope{interfaceInstanceNo: scope.InterfaceInstanceNo, configurationRevision: scope.InterfaceConfigurationRevision}, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, scope.Method, nil, ExistingBusinessRuntimeContextInput{IntentNo: "PFI_OTHER", OrderNo: scope.OrderNo, OrderItemNo: scope.OrderItemNo, ConfigurationSnapshotNo: scope.ConfigurationSnapshotNo})
	if issueErr != nil {
		t.Fatal(issueErr)
	}
	if _, err = invoker.Invoke(context.Background(), capregistry.Invocation{CallerPluginID: scope.PluginID, OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodFilesList, ResourceRef: wrongIntent.id, Payload: listPayload}); err == nil || strings.Contains(err.Error(), "CFU_") || strings.Contains(err.Error(), "private/") || strings.Contains(err.Error(), "storage.invalid") {
		t.Fatalf("cross-intent evidence error was not safely masked: %v", err)
	}
	request.Offset = 4
	requestPayload, _ = json.Marshal(request)
	second, err := invoker.Invoke(context.Background(), capregistry.Invocation{CallerPluginID: "ssl-provider", OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodFileRead, ResourceRef: dynamic.id, Payload: requestPayload})
	if err != nil {
		t.Fatal(err)
	}
	var secondChunk supplierinterfacecap.ConfigurationFileChunk
	_ = json.Unmarshal(second.Payload, &secondChunk)
	if string(secondChunk.Data) != "ef" || !secondChunk.EOF {
		t.Fatalf("second chunk: %#v", secondChunk)
	}

	released := issue()
	releaseRequest := supplierinterfacecap.ConfigurationFileReadRequest{FieldKey: "certificate", Sequence: 1, MaximumBytes: 1}
	releasePayload, _ := json.Marshal(releaseRequest)
	if _, err = invoker.Invoke(context.Background(), capregistry.Invocation{CallerPluginID: "ssl-provider", OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodFileRead, ResourceRef: released.id, Payload: releasePayload}); err != nil {
		t.Fatal(err)
	}
	service.releaseRuntimeContext(released.id)
	releaseRequest.Offset = 1
	releasePayload, _ = json.Marshal(releaseRequest)
	if _, err = invoker.Invoke(context.Background(), capregistry.Invocation{CallerPluginID: "ssl-provider", OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodFileRead, ResourceRef: released.id, Payload: releasePayload}); err == nil {
		t.Fatal("released context remained usable")
	}
	service.runtimeFiles.mu.Lock()
	remainingSessions := len(service.runtimeFiles.sessions)
	service.runtimeFiles.mu.Unlock()
	if remainingSessions != 0 {
		t.Fatalf("released context retained %d file streams", remainingSessions)
	}

	source := issue()
	client, err := supplierinterfacecap.NewSourceClient(invoker, invoker)
	if err != nil {
		t.Fatal(err)
	}
	declarations := pluginhost.NewDeclarations("ssl-provider")
	err = declarations.Providers().ProvideBusinessExtensionMethods(func(ctx context.Context, _ *capmodel.BusinessExtensionMethodInvocation) (*capmodel.BusinessExtensionMethodResult, error) {
		list, callErr := client.ListConfigurationFiles(ctx, source.id, "certificate")
		if callErr == nil && (len(list.Files) != 1 || list.Files[0].FileName != "request.csr") {
			t.Fatalf("source list: %#v", list)
		}
		if callErr == nil {
			_, callErr = client.ReadConfigurationFile(ctx, source.id, supplierinterfacecap.ConfigurationFileReadRequest{FieldKey: "certificate", Sequence: 1, MaximumBytes: 6})
		}
		return &capmodel.BusinessExtensionMethodResult{Version: capmodel.BusinessExtensionInvocationProtocolV1}, callErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = declarations.(pluginhost.SourcePluginDefinition).GetBusinessExtensionMethodHandler()(context.Background(), &capmodel.BusinessExtensionMethodInvocation{}); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeFilesExpireAndEnforceCallAndByteBounds(t *testing.T) {
	t.Parallel()
	store := newSupplierRuntimeContextStore(bytes.NewReader(bytes.Repeat([]byte{2}, 64)))
	item, err := store.issue(&capmodel.BusinessExtensionProviderEvidence{PluginID: "ssl-provider", Point: capmodel.BusinessExtensionProductFulfillment, AuthorizedMethods: []string{"provision"}}, dynamicCandidateScope{}, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, "provision", nil, ExistingBusinessRuntimeContextInput{IntentNo: "PFI_TEST", OrderNo: "ORD_TEST", OrderItemNo: "ORI_TEST", ConfigurationSnapshotNo: "SCS_TEST"})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	current := store.items[item.id]
	current.fileReadCalls = maximumRuntimeFileCalls
	store.items[item.id] = current
	store.mu.Unlock()
	if _, ok := store.reserveFileCall(item.id, "ssl-provider", "certificate", 1, 1); ok {
		t.Fatal("call limit was bypassed")
	}
	store.mu.Lock()
	current = store.items[item.id]
	current.fileReadCalls = 0
	current.fileReadBytes = maximumRuntimeFileBytes
	store.items[item.id] = current
	store.mu.Unlock()
	if _, ok := store.reserveFileCall(item.id, "ssl-provider", "certificate", 1, 1); ok {
		t.Fatal("byte limit was bypassed")
	}
	store.now = func() time.Time { return item.expiresAt }
	if _, ok := store.reserveFileCall(item.id, "ssl-provider", "certificate", 1, 0); ok {
		t.Fatal("expired context remained usable")
	}
}

func TestRuntimeFilesEnforceUniqueFieldAndFileBounds(t *testing.T) {
	t.Parallel()
	store := newSupplierRuntimeContextStore(bytes.NewReader(bytes.Repeat([]byte{3}, 64)))
	item, err := store.issue(&capmodel.BusinessExtensionProviderEvidence{PluginID: "ssl-provider", Point: capmodel.BusinessExtensionProductFulfillment, AuthorizedMethods: []string{"provision"}}, dynamicCandidateScope{}, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, "provision", nil, ExistingBusinessRuntimeContextInput{IntentNo: "PFI_TEST", OrderNo: "ORD_TEST", OrderItemNo: "ORI_TEST", ConfigurationSnapshotNo: "SCS_TEST"})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	current := store.items[item.id]
	for index := 0; index < maximumRuntimeFileFields; index++ {
		current.fileReadFields["field"+strconv.Itoa(index)] = struct{}{}
	}
	store.items[item.id] = current
	store.mu.Unlock()
	if _, ok := store.reserveFileCall(item.id, "ssl-provider", "overflow", 0, 0); ok {
		t.Fatal("field limit was bypassed")
	}

	store.mu.Lock()
	current = store.items[item.id]
	current.fileReadFields = map[string]struct{}{"certificate": {}}
	for index := 0; index < maximumRuntimeFileFiles; index++ {
		current.fileReadFiles["field"+strconv.Itoa(index/100)+"\x00"+strconv.Itoa(index%100+1)] = struct{}{}
	}
	store.items[item.id] = current
	store.mu.Unlock()
	if _, ok := store.reserveFileCall(item.id, "ssl-provider", "certificate", 1, 1); ok {
		t.Fatal("file limit was bypassed")
	}
}
