package productautomation

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const supplierRuntimeContextTTL = 10 * time.Second

type dynamicCandidateScope struct {
	versionNo                     string
	interfaceInstanceNo           string
	configurationRevision         int64
	resourceConfigurationRevision string
}

type supplierRuntimeContext struct {
	id                      string
	callerPluginID          string
	releaseVersion          string
	point                   string
	contractVersion         string
	releaseDigest           string
	declarationDigest       string
	interfaceInstanceNo     string
	configurationRevision   int64
	purpose                 capmodel.BusinessExtensionInvocationPurpose
	method                  string
	authorizedMethods       []string
	interfaceDeclaration    *capmodel.ConfigurationSchemaDeclaration
	configurationView       *supplierinterfacecap.ConfigurationView
	intentNo                string
	orderNo                 string
	orderItemNo             string
	configurationSnapshotNo string
	fileReadCalls           int
	fileReadBytes           int64
	fileReadFields          map[string]struct{}
	fileReadFiles           map[string]struct{}
	expiresAt               time.Time
}

type supplierRuntimeContextStore struct {
	mu     sync.Mutex
	items  map[string]supplierRuntimeContext
	random interface{ Read([]byte) (int, error) }
	now    func() time.Time
}

func newSupplierRuntimeContextStore(random interface{ Read([]byte) (int, error) }) *supplierRuntimeContextStore {
	return &supplierRuntimeContextStore{items: make(map[string]supplierRuntimeContext), random: random, now: time.Now}
}

func (s *supplierRuntimeContextStore) issue(exact *capmodel.BusinessExtensionProviderEvidence, scope dynamicCandidateScope, purpose capmodel.BusinessExtensionInvocationPurpose, method string, view *supplierinterfacecap.ConfigurationView, business ...ExistingBusinessRuntimeContextInput) (supplierRuntimeContext, error) {
	if s == nil || s.random == nil || exact == nil || exact.PluginID == "" || method == "" {
		return supplierRuntimeContext{}, gerror.New("supplier runtime context input is invalid")
	}
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(s.random, randomBytes); err != nil {
		return supplierRuntimeContext{}, gerror.Wrap(err, "generate supplier runtime context")
	}
	item := supplierRuntimeContext{
		id: "src_" + hex.EncodeToString(randomBytes), callerPluginID: exact.PluginID,
		releaseVersion: exact.ReleaseVersion, releaseDigest: exact.ReleaseDigest,
		point: exact.Point, contractVersion: exact.ContractVersion,
		declarationDigest: exact.DeclarationDigest, interfaceInstanceNo: scope.interfaceInstanceNo,
		configurationRevision: scope.configurationRevision, purpose: purpose, method: method,
		authorizedMethods:    append([]string(nil), exact.AuthorizedMethods...),
		interfaceDeclaration: exact.InterfaceConfiguration,
		configurationView:    cloneSupplierConfigurationView(view),
		expiresAt:            s.now().Add(supplierRuntimeContextTTL),
	}
	if len(business) > 1 {
		return supplierRuntimeContext{}, gerror.New("supplier runtime context business scope is invalid")
	}
	if len(business) == 1 {
		item.intentNo, item.orderNo, item.orderItemNo = business[0].IntentNo, business[0].OrderNo, business[0].OrderItemNo
		item.configurationSnapshotNo = business[0].ConfigurationSnapshotNo
		item.fileReadFields = make(map[string]struct{})
		item.fileReadFiles = make(map[string]struct{})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[item.id]; exists {
		return supplierRuntimeContext{}, gerror.New("supplier runtime context collision")
	}
	s.items[item.id] = item
	return item, nil
}

func (s *supplierRuntimeContextStore) remove(id string) {
	if s == nil || id == "" {
		return
	}
	s.mu.Lock()
	delete(s.items, id)
	s.mu.Unlock()
}

func (s *supplierRuntimeContextStore) get(id, callerPluginID string) (supplierRuntimeContext, bool) {
	if s == nil || id == "" || callerPluginID == "" {
		return supplierRuntimeContext{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return supplierRuntimeContext{}, false
	}
	if !s.now().Before(item.expiresAt) {
		delete(s.items, id)
		return supplierRuntimeContext{}, false
	}
	if item.callerPluginID != callerPluginID {
		return supplierRuntimeContext{}, false
	}
	return item, true
}

// RuntimeCapabilityInvoker serves the same owner-aware method to source and
// dynamic providers. The caller identity is supplied by the host, never JSON.
type RuntimeCapabilityInvoker struct {
	mu      sync.RWMutex
	service *serviceImpl
}

var _ capregistry.Invoker = (*RuntimeCapabilityInvoker)(nil)

func NewRuntimeCapabilityInvoker() *RuntimeCapabilityInvoker { return &RuntimeCapabilityInvoker{} }

// CheckPublicPurchaseAvailability exposes the already-bound core automation
// service to other core-owned startup graphs without introducing a second
// runtime registry or allowing a downstream plugin to reach internal state.
func (i *RuntimeCapabilityInvoker) CheckPublicPurchaseAvailability(
	ctx context.Context,
	versionNo string,
) (bool, error) {
	if i == nil {
		return false, gerror.New("supplier runtime capability is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil {
		return false, gerror.New("supplier runtime capability is unavailable")
	}
	return service.CheckPublicPurchaseAvailability(ctx, versionNo)
}

func (i *RuntimeCapabilityInvoker) ResolvePublicPurchaseCandidates(
	ctx context.Context,
	versionNo string,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []configurationcontract.ConfigurationValue,
) (configurationcontract.ResolvedCandidates, error) {
	if i == nil {
		return nil, gerror.New("supplier runtime capability is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil {
		return nil, gerror.New("supplier runtime capability is unavailable")
	}
	return service.ResolvePublicPurchaseCandidates(ctx, versionNo, schema, values)
}

func (i *RuntimeCapabilityInvoker) IssueExistingBusinessRuntimeContext(ctx context.Context, input ExistingBusinessRuntimeContextInput) (*ExistingBusinessRuntimeContext, error) {
	if i == nil {
		return nil, gerror.New("supplier runtime capability is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil {
		return nil, gerror.New("supplier runtime capability is unavailable")
	}
	return service.IssueExistingBusinessRuntimeContext(ctx, input)
}

func (i *RuntimeCapabilityInvoker) Bind(service Service) error {
	concrete, ok := service.(*serviceImpl)
	if i == nil || !ok || concrete == nil {
		return gerror.New("supplier runtime capability requires product automation service")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.service != nil {
		return gerror.New("supplier runtime capability is already bound")
	}
	i.service = concrete
	return nil
}

func (i *RuntimeCapabilityInvoker) Invoke(ctx context.Context, invocation capregistry.Invocation) (*capregistry.InvocationResult, error) {
	if i == nil {
		return nil, gerror.New("supplier runtime capability is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil || invocation.OwnerPluginID != supplierinterfacecap.OwnerPluginID ||
		invocation.Service != supplierinterfacecap.ServiceName || invocation.Version != supplierinterfacecap.ContractVersion ||
		invocation.ResourceRef == "" || invocation.Table != "" {
		return nil, gerror.New("supplier runtime capability invocation is not authorized")
	}
	var response any
	switch invocation.Method {
	case supplierinterfacecap.MethodConfigurationGet:
		if string(invocation.Payload) != "{}" {
			return nil, gerror.New("supplier runtime capability invocation is not authorized")
		}
		view, err := service.runtimeConfigurationView(ctx, invocation.CallerPluginID, invocation.ResourceRef)
		if err != nil {
			return nil, err
		}
		response = view
	case supplierinterfacecap.MethodFilesList:
		var request supplierinterfacecap.ConfigurationFileListRequest
		if strictRuntimeJSON(invocation.Payload, &request) != nil {
			return nil, gerror.New("supplier runtime file request is unavailable")
		}
		list, err := service.runtimeFileList(ctx, invocation.CallerPluginID, invocation.ResourceRef, request.FieldKey)
		if err != nil {
			return nil, err
		}
		response = list
	case supplierinterfacecap.MethodFileRead:
		var request supplierinterfacecap.ConfigurationFileReadRequest
		if strictRuntimeJSON(invocation.Payload, &request) != nil {
			return nil, gerror.New("supplier runtime file request is unavailable")
		}
		chunk, err := service.runtimeFileRead(ctx, invocation.CallerPluginID, invocation.ResourceRef, request)
		if err != nil {
			return nil, err
		}
		response = chunk
	case supplierinterfacecap.MethodOutboundHTTP:
		var request supplierinterfacecap.OutboundHTTPRequest
		if strictRuntimeJSON(invocation.Payload, &request) != nil {
			return nil, gerror.New("supplier runtime outbound request is unavailable")
		}
		result, err := service.runtimeOutboundHTTP(ctx, invocation.CallerPluginID, invocation.ResourceRef, request)
		if err != nil {
			return nil, err
		}
		response = result
	case supplierinterfacecap.MethodOutboundSession:
		var request supplierinterfacecap.OutboundSessionRequest
		if strictRuntimeJSON(invocation.Payload, &request) != nil {
			return nil, gerror.New("supplier runtime outbound session request is unavailable")
		}
		result, err := service.runtimeOutboundSession(ctx, invocation.CallerPluginID, invocation.ResourceRef, request)
		if err != nil {
			return nil, err
		}
		response = result
	default:
		return nil, gerror.New("supplier runtime capability invocation is not authorized")
	}
	payload, err := json.Marshal(response)
	maximum := supplierinterfacecap.MaximumPayloadBytes
	if invocation.Method == supplierinterfacecap.MethodFileRead {
		maximum = supplierinterfacecap.MaximumFilePayloadBytes
	} else if invocation.Method == supplierinterfacecap.MethodOutboundHTTP || invocation.Method == supplierinterfacecap.MethodOutboundSession {
		maximum = supplierinterfacecap.MaximumOutboundPayload
	}
	if err != nil || len(payload) > maximum {
		return nil, gerror.New("supplier runtime configuration response is unavailable")
	}
	return &capregistry.InvocationResult{Payload: payload}, nil
}

func strictRuntimeJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return gerror.New("supplier runtime request has trailing data")
	}
	return nil
}

// AccessSecret is source-only: the caller identity comes from pluginhost's
// private handler context and plaintext never enters a capability payload.
func (i *RuntimeCapabilityInvoker) AccessSecret(ctx context.Context, contextID, fieldKey string, consume func([]byte) error) error {
	if i == nil || ctx == nil || consume == nil {
		return gerror.New("supplier runtime secret access is unavailable")
	}
	callerPluginID, ok := pluginhost.SourceBusinessExtensionCallerPluginID(ctx)
	if !ok {
		return gerror.New("supplier runtime source identity is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil {
		return gerror.New("supplier runtime secret access is unavailable")
	}
	return service.runtimeSecretAccess(ctx, callerPluginID, contextID, fieldKey, consume)
}

func (s *serviceImpl) runtimeSecretAccess(ctx context.Context, callerPluginID, contextID, fieldKey string, consume func([]byte) error) error {
	runtimeContext, ok := s.runtimeContexts.get(contextID, callerPluginID)
	if !ok || s.secretAccessor == nil || runtimeContext.interfaceInstanceNo == "" || runtimeContext.configurationRevision < 1 ||
		fieldKey == "" || fieldKey != strings.TrimSpace(fieldKey) || len(fieldKey) > 64 || consume == nil ||
		!containsString(runtimeContext.authorizedMethods, runtimeContext.method) || !runtimeSecretField(runtimeContext.interfaceDeclaration, fieldKey) {
		return gerror.New("supplier runtime secret access is unavailable")
	}
	value, err := s.database.GetValue(ctx, `SELECT reference.secret_ref
FROM plugin_idc_core_product_automation_interface_configuration_revision revision
JOIN plugin_idc_core_product_automation_interface_secret_reference reference ON reference.configuration_revision_id=revision.id
WHERE revision.tenant_id=0 AND revision.interface_instance_no=? AND revision.revision=? AND reference.field_key=?
LIMIT 1`, runtimeContext.interfaceInstanceNo, runtimeContext.configurationRevision, fieldKey)
	if err != nil || value == nil || strings.TrimSpace(value.String()) == "" {
		return gerror.New("supplier runtime secret evidence is unavailable")
	}
	handle, err := secretcap.NewHandle(value.String())
	if err != nil {
		return gerror.New("supplier runtime secret evidence is unavailable")
	}
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		return gerror.New("supplier runtime secret purpose is unavailable")
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return gerror.New("supplier runtime secret request is unavailable")
	}
	return s.secretAccessor.Access(ctx, request, consume)
}

func runtimeSecretField(schema *capmodel.ConfigurationSchemaDeclaration, fieldKey string) bool {
	if schema == nil {
		return false
	}
	for _, field := range schema.Fields {
		if field != nil && field.Key == fieldKey {
			return field.Type == capmodel.ConfigurationFieldSecret
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *serviceImpl) runtimeConfigurationView(ctx context.Context, callerPluginID, contextID string) (*supplierinterfacecap.ConfigurationView, error) {
	_ = ctx
	runtimeContext, ok := s.runtimeContexts.get(contextID, callerPluginID)
	if !ok || runtimeContext.interfaceInstanceNo == "" || runtimeContext.configurationRevision < 1 || runtimeContext.configurationView == nil {
		return nil, gerror.New("supplier runtime context is unavailable")
	}
	return cloneSupplierConfigurationView(runtimeContext.configurationView), nil
}

func (s *serviceImpl) loadRuntimeConfigurationView(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, scope dynamicCandidateScope) (*supplierinterfacecap.ConfigurationView, error) {
	if exact == nil || scope.interfaceInstanceNo == "" || scope.configurationRevision < 1 {
		return nil, nil
	}
	var row *supplierRuntimeConfigurationRow
	if err := s.database.GetScan(ctx, &row, supplierRuntimeConfigurationQuery, scope.interfaceInstanceNo, scope.configurationRevision); err != nil || row == nil ||
		row.PluginID != exact.PluginID || row.PluginReleaseVersion != exact.ReleaseVersion || row.ReleaseDigest != exact.ReleaseDigest ||
		row.BindingDeclarationDigest != exact.DeclarationDigest {
		return nil, gerror.New("supplier runtime configuration evidence is unavailable")
	}
	declaration, err := decodeConfigurationDeclaration(row.DeclarationSnapshot)
	expectedDeclaration, expectedErr := configurationDeclarationJSON(exact.InterfaceConfiguration)
	if err != nil || expectedErr != nil || !sameExactConfiguration(declaration, exact.InterfaceConfiguration) ||
		configurationDigest(expectedDeclaration) != row.DeclarationDigest {
		return nil, gerror.New("supplier runtime configuration declaration is unavailable")
	}
	values, err := configurationcontract.DecodeCompiledValues(declaration, []byte(row.PublicValueSnapshot))
	if err != nil {
		return nil, gerror.New("supplier runtime configuration values are unavailable")
	}
	projected := make([]supplierinterfacecap.ConfigurationValue, 0, len(values))
	fieldTypes := make(map[string]string)
	if exact.InterfaceConfiguration != nil {
		fieldTypes = make(map[string]string, len(exact.InterfaceConfiguration.Fields))
		for _, field := range exact.InterfaceConfiguration.Fields {
			if field != nil {
				fieldTypes[field.Key] = field.Type
			}
		}
	}
	for _, value := range values {
		if fieldTypes[value.FieldKey] == capmodel.ConfigurationFieldSecret {
			continue
		}
		projected = append(projected, supplierinterfacecap.ConfigurationValue{FieldKey: value.FieldKey,
			TextValue: value.TextValue, IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue,
			BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue,
			OptionTokens: append([]string(nil), value.OptionTokens...), TreeOptionTokens: append([]string(nil), value.TreeOptionTokens...),
			AddressOptionTokens: append([]string(nil), value.AddressOptionTokens...),
		})
	}
	view := &supplierinterfacecap.ConfigurationView{Values: projected}
	if err = supplierinterfacecap.ValidateConfigurationView(view); err != nil {
		return nil, err
	}
	return view, nil
}

type supplierRuntimeConfigurationRow struct {
	PluginID                 string `orm:"plugin_id"`
	PluginReleaseVersion     string `orm:"plugin_release_version"`
	ReleaseDigest            string `orm:"release_digest"`
	BindingDeclarationDigest string `orm:"binding_declaration_digest"`
	DeclarationSnapshot      string `orm:"declaration_snapshot"`
	PublicValueSnapshot      string `orm:"public_value_snapshot"`
	DeclarationDigest        string `orm:"declaration_digest"`
}

const supplierRuntimeConfigurationQuery = `SELECT instance.plugin_id,instance.plugin_release_version,
instance.release_digest,instance.declaration_digest AS binding_declaration_digest,
revision.declaration_snapshot,revision.public_value_snapshot,revision.declaration_digest
FROM plugin_idc_core_product_automation_interface_instance instance
JOIN plugin_idc_core_product_automation_interface_configuration_revision revision
  ON revision.tenant_id=instance.tenant_id AND revision.interface_instance_no=instance.interface_instance_no
WHERE instance.tenant_id=0 AND instance.interface_instance_no=? AND revision.revision=?`

func cloneSupplierConfigurationView(view *supplierinterfacecap.ConfigurationView) *supplierinterfacecap.ConfigurationView {
	if view == nil {
		return nil
	}
	result := &supplierinterfacecap.ConfigurationView{Values: make([]supplierinterfacecap.ConfigurationValue, len(view.Values))}
	for index, value := range view.Values {
		result.Values[index] = value
		result.Values[index].OptionTokens = append([]string(nil), value.OptionTokens...)
		result.Values[index].TreeOptionTokens = append([]string(nil), value.TreeOptionTokens...)
		result.Values[index].AddressOptionTokens = append([]string(nil), value.AddressOptionTokens...)
	}
	return result
}
