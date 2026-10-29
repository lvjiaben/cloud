// Package interfacecandidate resolves server configuration candidates through
// the resource owner's pfi-to-channel mapping without exposing credentials.
package interfacecandidate

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

const (
	maximumCandidateFields = 16
	maximumCandidateItems  = 512
	maximumCandidateText   = 256
)

type outboundFactory func(
	context.Context,
	channelconfig.Configuration,
	string,
	[]resourcecap.SecretBinding,
) (pluginhost.ConfidentialOutboundHandler, func(), error)

type dependencies struct {
	database   gdb.DB
	extensions pluginhost.BusinessExtensionRegistrar
	outbound   outboundFactory
	files      FileTransferFactory
}

// LazySource can be declared during plugin init and bound when host services
// become available. Calls fail closed until the complete owner runtime exists.
type LazySource struct {
	mu   sync.RWMutex
	deps *dependencies
}

func NewLazy() *LazySource { return &LazySource{} }

func (source *LazySource) Bind(
	database gdb.DB,
	extensions pluginhost.BusinessExtensionRegistrar,
	outbound func(
		context.Context,
		channelconfig.Configuration,
		string,
		[]resourcecap.SecretBinding,
	) (pluginhost.ConfidentialOutboundHandler, func(), error),
) error {
	if source == nil || database == nil || extensions == nil || outbound == nil {
		return gerror.New("resource interface candidate dependencies are required")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.deps != nil {
		return gerror.New("resource interface candidate source is already bound")
	}
	source.deps = &dependencies{database: database, extensions: extensions, outbound: outboundFactory(outbound)}
	return nil
}

func (source *LazySource) ResolveCandidates(
	ctx context.Context,
	request resourceinterfaceprojectioncap.CandidateRequest,
) (*resourceinterfaceprojectioncap.CandidateResult, error) {
	peer := request.Method == "peer.configuration.candidates"
	if peer {
		request.Method = resourcecap.DynamicProviderMethodConfigurationCandidates
	}
	if request.Method != resourcecap.DynamicProviderMethodConfigurationCandidates {
		return nil, gerror.New("resource interface candidate method is unavailable")
	}
	if !peer && source != nil && ctx != nil && ctx.Err() == nil && resourceinterfaceprojectioncap.ValidateCandidateRequest(request) == nil {
		source.mu.RLock()
		deps := source.deps
		source.mu.RUnlock()
		if deps != nil {
			current, err := loadBinding(ctx, deps.database, request.InterfaceInstanceNo)
			if err != nil {
				return nil, err
			}
			if current.PluginID == "idc-zjmfcloud" {
				return source.readCandidateSnapshot(ctx, deps, request, current)
			}
		}
	}
	response, evidence, err := source.invokeRead(ctx, request, []string{"StatusCode", "Data", "Authenticated", "Fields", "FieldKey", "Items", "Value", "ParentValue", "OptionToken", "LabelZhCN", "LabelEnUS", "Relationships", "ParentFieldKey", "ChildFieldKey", "Links", "ChildValue"})
	if err != nil {
		return nil, err
	}
	if peer && (evidence == nil || evidence.PluginID != "idc-peer") {
		return nil, gerror.New("peer candidate connector is invalid")
	}
	if !peer {
		return projectCandidateResult(response)
	}
	return projectCandidates(response, peer)
}

// invokeRead preserves one exact resource-owner channel/evidence/confidential
// outbound path for the bounded candidate and staff-only catalog projections.
func (source *LazySource) invokeRead(
	ctx context.Context, request resourceinterfaceprojectioncap.CandidateRequest, resultFields []string,
) ([]byte, *resourceinterfaceprojectioncap.CatalogReadEvidence, error) {
	if source == nil || ctx == nil || ctx.Err() != nil ||
		resourceinterfaceprojectioncap.ValidateCandidateRequest(request) != nil {
		return nil, nil, gerror.New("resource interface candidates are unavailable")
	}
	switch request.Method {
	case resourcecap.DynamicProviderMethodConfigurationCandidates, resourcecap.DynamicProviderMethodListCatalog, resourcecap.DynamicProviderMethodReadCatalogProduct, resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration, resourcecap.DynamicProviderMethodCatalogBillingTerms:
	default:
		return nil, nil, gerror.New("resource interface read method is unavailable")
	}
	source.mu.RLock()
	deps := source.deps
	source.mu.RUnlock()
	if deps == nil {
		return nil, nil, gerror.New("resource interface candidates are unavailable")
	}
	binding, err := loadBinding(ctx, deps.database, request.InterfaceInstanceNo)
	if err != nil {
		return nil, nil, err
	}
	if request.ExpectedConfigurationRevision != "" && request.ExpectedConfigurationRevision != binding.ConfigurationRevision {
		return nil, nil, gerror.New("resource interface configuration revision changed")
	}
	if !contains(binding.AuthorizedMethods, request.Method) {
		return nil, nil, gerror.New("resource interface candidate method is unavailable")
	}
	evidence, err := deps.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: binding.PluginID, ReleaseVersion: binding.PluginReleaseVersion,
		Point: resourcecap.DynamicProviderPoint, ContractVersion: binding.ContractVersion,
		ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest,
	})
	if err != nil || !matchesResolvedEvidence(ctx, deps.extensions, binding, evidence, request.Method) {
		return nil, nil, gerror.New("resource interface candidate connector evidence is unavailable")
	}
	reader, err := channelconfig.NewReader(deps.database)
	if err != nil {
		return nil, nil, err
	}
	configuration, err := reader.ReadRevision(ctx, binding.ChannelNo, binding.ConfigurationRevision)
	if err != nil || !matchesConfiguration(binding, configuration) {
		return nil, nil, gerror.New("resource interface candidate channel is unavailable")
	}
	settings, err := resourcecap.ProjectPublicInterfaceSettings(configuration.Config.Settings, true)
	if err != nil {
		return nil, nil, gerror.New("resource interface candidate settings are unavailable")
	}
	payload, err := resourcecap.NewDynamicProviderInvocationEnvelope(settings, request.Values)
	if err != nil {
		return nil, nil, gerror.New("resource interface candidate request is invalid")
	}
	handler, cleanup, err := deps.outbound(ctx, configuration, request.Method, nil)
	if err != nil || handler == nil || cleanup == nil {
		return nil, nil, gerror.New("resource interface candidate outbound handler is unavailable")
	}
	defer cleanup()

	var response []byte
	err = deps.extensions.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{
		ProviderPluginID: binding.PluginID, Point: resourcecap.DynamicProviderPoint,
		ContractVersion: binding.ContractVersion, ProviderMethod: request.Method,
		OutboundMethod: "resource-provider." + request.Method,
		ReleaseVersion: evidence.ReleaseVersion, ReleaseDigest: evidence.ReleaseDigest,
		DeclarationDigest: evidence.DeclarationDigest, InterfaceInstanceID: binding.ChannelNo,
		Purpose:             capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		ExpiresAfter:        30 * time.Second,
		AllowedResultFields: resultFields,
		Handler:             handler,
	}, func(callCtx context.Context, runtimeContext *capmodel.BusinessExtensionRuntimeContext) error {
		result, invokeErr := deps.extensions.InvokeProviderMethod(callCtx, pluginhost.BusinessExtensionMethodInvokeInput{
			PluginID: binding.PluginID, Point: resourcecap.DynamicProviderPoint,
			ContractVersion: binding.ContractVersion, Method: request.Method,
			ReleaseVersion: evidence.ReleaseVersion, ReleaseDigest: evidence.ReleaseDigest,
			DeclarationDigest: binding.DeclarationDigest,
			Purpose:           capmodel.BusinessExtensionInvocationPurposeNewBusiness,
			RuntimeContext:    runtimeContext, Payload: payload,
		})
		if invokeErr != nil || result == nil {
			diagnostic := "unclassified"
			if invokeErr != nil {
				switch gerror.Current(invokeErr).Error() {
				case "dynamic business extension provider invocation failed":
					diagnostic = "connector_runtime_failed"
				case "dynamic business extension provider rejected the governed invocation":
					diagnostic = "connector_rejected_invocation"
				case "dynamic business extension provider returned an invalid response envelope":
					diagnostic = "connector_invalid_envelope"
				}
			}
			if businessError, ok := bizerr.As(invokeErr); ok {
				diagnostic = businessError.RuntimeCode()
			}
			logger.Warningf(callCtx, "resource candidate provider invocation failed code=%s", diagnostic)
			return gerror.New("resource interface candidate connector invocation failed")
		}
		response = append([]byte(nil), result.Payload...)
		return nil
	})
	if err != nil {
		return nil, nil, gerror.New("resource interface candidate connector invocation failed")
	}
	current, err := loadBinding(ctx, deps.database, request.InterfaceInstanceNo)
	if err != nil || !sameReadBinding(binding, current) {
		return nil, nil, gerror.New("resource interface changed during catalog read")
	}
	receipt := binding.readEvidence(request.InterfaceInstanceNo)
	if err = receipt.Validate(); err != nil {
		return nil, nil, err
	}
	return response, receipt, nil
}

type binding struct {
	ChannelNo, PluginID, PluginReleaseVersion, ContractVersion     string
	ReleaseDigest, DeclarationDigest                               string
	AuthorizedMethods                                              []string
	ConfigurationRevision                                          string
	ProductPluginID, ProductReleaseVersion, ProductContractVersion string
	ProductReleaseDigest, ProductDeclarationDigest                 string
	ProductAuthorizedMethods                                       []string
}

func loadBinding(ctx context.Context, database gdb.DB, interfaceNo string) (binding, error) {
	return loadBusinessBinding(ctx, database, interfaceNo, true)
}

func loadBusinessBinding(ctx context.Context, database gdb.DB, interfaceNo string, newBusiness bool) (binding, error) {
	var value binding
	model := database.Model("plugin_idc_resource_interface_channel_binding b").Safe().Ctx(ctx).
		InnerJoin("plugin_idc_resource_channel c", "c.tenant_id=b.tenant_id AND c.channel_no=b.channel_no").
		Fields("b.channel_no,b.resource_plugin_id AS plugin_id",
			"b.resource_plugin_release_version AS plugin_release_version",
			"b.resource_contract_version AS contract_version",
			"b.resource_authorized_methods AS authorized_methods",
			"b.resource_release_digest AS release_digest",
			"b.resource_declaration_digest AS declaration_digest",
			"b.current_configuration_revision AS configuration_revision",
			"b.product_plugin_id,b.product_plugin_release_version AS product_release_version,b.product_contract_version,b.product_authorized_methods,b.product_release_digest,b.product_declaration_digest").
		Where("b.tenant_id", idccap.DefaultTenantID).
		Where("b.interface_instance_no", interfaceNo).
		Where("c.status_code", "enabled").WhereNull("c.deleted_at").Limit(1)
	if newBusiness {
		model = model.Where("b.new_business_state", resourceinterfaceprojectioncap.StateAccepting)
	}
	err := model.Scan(&value)
	if err != nil || value.ChannelNo == "" {
		return binding{}, gerror.New("resource interface candidates are unavailable")
	}
	return value, nil
}

// readEvidence uses only the owner database binding, never guest response fields.
func (value binding) readEvidence(interfaceNo string) *resourceinterfaceprojectioncap.CatalogReadEvidence {
	return &resourceinterfaceprojectioncap.CatalogReadEvidence{
		InterfaceInstanceNo: interfaceNo, OwnerID: resourceinterfaceprojectioncap.OwnerID,
		ResourceChannelRef: value.ChannelNo, InterfaceConfigurationRevision: value.ConfigurationRevision,
		PluginID: value.ProductPluginID, PluginReleaseVersion: value.ProductReleaseVersion,
		ContractVersion: value.ProductContractVersion, AuthorizedMethods: append([]string(nil), value.ProductAuthorizedMethods...),
		ReleaseDigest: value.ProductReleaseDigest, DeclarationDigest: value.ProductDeclarationDigest,
	}
}

// sameReadBinding rejects configuration, provider and state changes observed
// after outbound reading. loadBinding already excludes stopped/deleted targets.
func sameReadBinding(left, right binding) bool {
	return left.ChannelNo == right.ChannelNo && left.ConfigurationRevision == right.ConfigurationRevision &&
		left.PluginID == right.PluginID && left.PluginReleaseVersion == right.PluginReleaseVersion &&
		left.ContractVersion == right.ContractVersion && left.ReleaseDigest == right.ReleaseDigest && left.DeclarationDigest == right.DeclarationDigest &&
		slices.Equal(left.AuthorizedMethods, right.AuthorizedMethods) && left.ProductPluginID == right.ProductPluginID &&
		left.ProductReleaseVersion == right.ProductReleaseVersion && left.ProductContractVersion == right.ProductContractVersion &&
		left.ProductReleaseDigest == right.ProductReleaseDigest && left.ProductDeclarationDigest == right.ProductDeclarationDigest &&
		slices.Equal(left.ProductAuthorizedMethods, right.ProductAuthorizedMethods)
}

func matchesConfiguration(value binding, configuration channelconfig.Configuration) bool {
	return configuration.PluginID == value.PluginID &&
		configuration.ReleaseVersion == value.PluginReleaseVersion &&
		configuration.ContractVersion == value.ContractVersion &&
		configuration.ReleaseDigest == value.ReleaseDigest &&
		configuration.DeclarationDigest == value.DeclarationDigest
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func projectCandidateResult(payload []byte) (*resourceinterfaceprojectioncap.CandidateResult, error) {
	return projectCandidates(payload, false)
}

var peerCandidateKey = regexp.MustCompile(`^k_[a-f0-9]{48}$`)

func projectCandidates(payload []byte, peer bool) (*resourceinterfaceprojectioncap.CandidateResult, error) {
	var value resourceinterfaceprojectioncap.CandidateResult
	fieldLimit := maximumCandidateFields
	if peer {
		fieldLimit = 128
	}
	if len(payload) == 0 || len(payload) > 1<<20 || json.Unmarshal(payload, &value) != nil ||
		(!peer && len(value.Fields) == 0) || len(value.Fields) > fieldLimit {
		return nil, gerror.New("resource interface candidate result is invalid")
	}
	allowedKeys := make(map[string]struct{}, len(resourcecap.ServerConfigurationKeys()))
	for _, key := range resourcecap.ServerConfigurationKeys() {
		allowedKeys[key] = struct{}{}
	}
	seenFields := make(map[string]struct{}, len(value.Fields))
	total := 0
	for fieldIndex := range value.Fields {
		field := &value.Fields[fieldIndex]
		_, ok := allowedKeys[field.FieldKey]
		if peer {
			ok = peerCandidateKey.MatchString(field.FieldKey)
		}
		if !ok || (!peer && len(field.Items) == 0) {
			return nil, gerror.New("resource interface candidate field is invalid")
		}
		if _, exists := seenFields[field.FieldKey]; exists {
			return nil, gerror.New("resource interface candidate fields are duplicated")
		}
		seenFields[field.FieldKey] = struct{}{}
		seenValues := make(map[string]struct{}, len(field.Items))
		for itemIndex := range field.Items {
			item := &field.Items[itemIndex]
			if !safeText(item.Value) || !safeText(item.LabelZhCN) || !safeText(item.LabelEnUS) {
				return nil, gerror.New("resource interface candidate item is invalid")
			}
			if peer && (len(item.OptionToken) == 0 || len(item.OptionToken) > 4096 || (item.ParentValue != "" && !safeText(item.ParentValue))) {
				return nil, gerror.New("peer candidate mapping is invalid")
			}
			if _, exists := seenValues[item.Value]; exists {
				return nil, gerror.New("resource interface candidate items are duplicated")
			}
			seenValues[item.Value] = struct{}{}
			total++
			if total > maximumCandidateItems {
				return nil, gerror.New("resource interface candidate result is too large")
			}
		}
	}
	for _, relation := range value.Relationships {
		_, parentAllowed := allowedKeys[relation.ParentFieldKey]
		_, childAllowed := allowedKeys[relation.ChildFieldKey]
		if peer {
			parentAllowed, childAllowed = peerCandidateKey.MatchString(relation.ParentFieldKey), peerCandidateKey.MatchString(relation.ChildFieldKey)
		}
		if !parentAllowed || !childAllowed {
			return nil, gerror.New("resource interface candidate relationship field is invalid")
		}
	}
	if err := value.ValidateRelationships(); err != nil {
		return nil, err
	}
	return &value, nil
}

func safeText(value string) bool {
	if strings.TrimSpace(value) != value || value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximumCandidateText {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

// matchesResolvedEvidence follows the current enabled provider identity and
// current method authority. Historical packages are not an execution dependency.
func matchesResolvedEvidence(_ context.Context, _ pluginhost.BusinessExtensionRegistrar, value binding, evidence *capmodel.BusinessExtensionProviderEvidence, method string) bool {
	return evidence != nil && evidence.PluginID == value.PluginID && evidence.Point == resourcecap.DynamicProviderPoint && evidence.ContractVersion == value.ContractVersion && len(evidence.ReleaseDigest) == 64 && len(evidence.DeclarationDigest) == 64 && contains(evidence.AuthorizedMethods, method)
}
