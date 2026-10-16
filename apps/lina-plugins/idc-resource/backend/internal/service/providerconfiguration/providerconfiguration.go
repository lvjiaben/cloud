// Package providerconfiguration owns supplier interface configuration and pool
// administration, with explicit staff authorization and plugin-owned projections.
package providerconfiguration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	providerspi "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
)

const (
	PermissionList  = "idc:admin:resource:configuration:list"
	PermissionWrite = "idc:admin:resource:configuration:write"
)

var (
	CodeRefreshReferenced = bizerr.MustDefine("IDC_RESOURCE_CONFIGURATION_REFRESH_REFERENCED", "This interface has historical use; create a new interface for the updated connector", gcode.CodeInvalidOperation)
	CodeInvalid           = bizerr.MustDefine("IDC_RESOURCE_CONFIGURATION_INVALID", "The Provider configuration request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible     = bizerr.MustDefine("IDC_RESOURCE_CONFIGURATION_NOT_ACCESSIBLE", "The Provider configuration target is not accessible", gcode.CodeNotAuthorized)
	CodeConflict          = bizerr.MustDefine("IDC_RESOURCE_CONFIGURATION_CONFLICT", "The Provider configuration changed; refresh and try again", gcode.CodeInvalidOperation)
	CodeUnavailable       = bizerr.MustDefine("IDC_RESOURCE_CONFIGURATION_UNAVAILABLE", "The Provider configuration service is temporarily unavailable", gcode.CodeInternalError)
)

type Channel struct {
	TLSCertificateSHA256                                         string
	ChannelNo, InterfaceInstanceNo, ProviderID, ProviderPluginID string
	DisplayName, Endpoint, Status                                string
	Version                                                      int64
	NonSecretSettings                                            json.RawMessage
	SecretFields                                                 []SecretFieldStatus
	InterfaceConfiguration                                       *capmodel.ConfigurationSchemaDeclaration
	ProviderContractVersion, ProviderReleaseVersion              string
	ProviderReleaseDigest, ProviderDeclarationDigest             string
	VerificationHealthCode, VerificationSummaryCode              string
	VerificationObservedAt                                       *time.Time
	Usage                                                        *resourceinterfaceprojectioncap.Usage
	CatalogAvailable                                             bool
}

type SecretFieldStatus struct {
	Key       string
	Required  bool
	Ready     bool
	UpdatedAt time.Time
}

type channelConfigurationStatus struct {
	SecretFields           []SecretFieldStatus
	InterfaceConfiguration *capmodel.ConfigurationSchemaDeclaration
}

type Pool struct {
	PoolNo, ChannelNo, DisplayName, ResourceKind, Status string
	ChannelName                                          string
	RoutingPriority, RoutingWeight                       int
	Version                                              int64
}

type ProviderCandidate struct {
	PluginID, ProviderID, DisplayName, ReleaseVersion string
	InterfaceConfiguration                            *capmodel.ConfigurationSchemaDeclaration
}

type Snapshot struct {
	Channels           []Channel
	Pools              []Pool
	ProviderCandidates []ProviderCandidate
	ChannelTotal       int
	PoolTotal          int
}

// ListInput bounds both independently paged directories. AvailableChannelsOnly
// and ChannelKeyword also serve the pool form's remote channel selector.
type ListInput struct {
	ChannelPage, ChannelPageSize int
	PoolPage, PoolPageSize       int
	ChannelKeyword               string
	AvailableChannelsOnly        bool
}

type channelListRow struct {
	TLSCertificateSHA256                                                 string
	ChannelNo, InterfaceInstanceNo, ProviderID, ProviderPluginID         string
	DisplayName, Endpoint, StatusCode                                    string
	Version                                                              int64
	NonSecretSettings                                                    string
	ProviderRuntimeType, ResourceContractVersion, ResourceReleaseVersion string
	ResourceReleaseDigest, ResourceDeclarationDigest                     string
	VerificationHealthCode, VerificationSummaryCode                      string
	VerificationObservedAt                                               *time.Time
	ResourceAuthorizedMethods                                            []string
}
type ChannelInput struct {
	RefreshProvider                                            bool
	TLSCertificateSHA256                                       *string
	ChannelNo, ProviderPluginID, DisplayName, Endpoint, Status string
	Version                                                    int64
	NonSecretSettings                                          json.RawMessage
}
type PoolInput struct {
	PoolNo, ChannelNo, DisplayName, ResourceKind, Status string
	RoutingPriority, RoutingWeight                       int
	Version                                              int64
}

type Service interface {
	// List authorizes the merchant scope before querying and returns accurate
	// counts with independently paged channel and pool rows; zero sizes use 20.
	List(context.Context, ListInput) (*Snapshot, error)
	CreateChannel(context.Context, ChannelInput) (*Channel, error)
	UpdateChannel(context.Context, ChannelInput) (*Channel, error)
	CreatePool(context.Context, PoolInput) (*Pool, error)
	UpdatePool(context.Context, PoolInput) (*Pool, error)
}

type service struct {
	db         gdb.DB
	runtime    managementcap.Runtime
	provider   resourcecap.ProviderKey
	extensions pluginhost.BusinessExtensionRegistrar
	usage      resourceinterfaceprojectioncap.UsageReader
	projection *interfaceprojection.Worker
}

// New binds database, staff runtime, declarations and read-only local usage once.
func New(
	db gdb.DB,
	runtime managementcap.Runtime,
	declarations []providerspi.Declaration,
	usage resourceinterfaceprojectioncap.UsageReader,
	projection *interfaceprojection.Worker,
	extensions ...pluginhost.BusinessExtensionRegistrar,
) (Service, error) {
	if db == nil || usage == nil || projection == nil || managementcap.ValidateRuntime(runtime) != nil {
		return nil, gerror.New("Provider configuration dependencies are invalid")
	}
	key, err := selectSoleServerProvider(declarations)
	if err != nil {
		return nil, err
	}
	var extensionRegistrar pluginhost.BusinessExtensionRegistrar
	if len(extensions) > 0 {
		extensionRegistrar = extensions[0]
	}
	if key.PluginID == "" && extensionRegistrar == nil {
		return nil, gerror.New("Provider configuration requires an available Provider extension")
	}
	return &service{db: db, runtime: runtime, provider: key, extensions: extensionRegistrar, usage: usage, projection: projection}, nil
}

func selectSoleServerProvider(declarations []providerspi.Declaration) (resourcecap.ProviderKey, error) {
	var keys []resourcecap.ProviderKey
	for _, item := range declarations {
		if item.ProvisionParameters == nil {
			continue
		}
		if item.Key.Validate() != nil || item.Key.ContractVersion != resourcecap.ContractVersion {
			return resourcecap.ProviderKey{}, gerror.New("Provider configuration declaration is invalid")
		}
		keys = append(keys, item.Key)
	}
	if len(keys) > 1 {
		return resourcecap.ProviderKey{}, gerror.New("Provider configuration has ambiguous source Provider declarations")
	}
	if len(keys) == 0 {
		return resourcecap.ProviderKey{}, nil
	}
	return keys[0], nil
}

func (s *service) authorize(ctx context.Context, permission, target string) (int, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: permission, TargetNo: target})
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

func (s *service) dynamicProviderCandidates(ctx context.Context) ([]ProviderCandidate, error) {
	if s.extensions == nil {
		return []ProviderCandidate{}, nil
	}
	page, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point: resourcecap.DynamicProviderPoint, ContractVersion: resourcecap.DynamicProviderContractVersion,
		Page: capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100},
	})
	if err != nil || page == nil || len(page.Items) > 100 {
		return nil, bizerr.WrapCode(errOrLimit(err), CodeUnavailable)
	}
	return providerCandidatesFromEvidence(page.Items), nil
}

func (s *service) channelConfigurationStatuses(ctx context.Context, channels []channelListRow) (map[string]channelConfigurationStatus, error) {
	result := make(map[string]channelConfigurationStatus, len(channels))
	if len(channels) == 0 {
		return result, nil
	}
	var activeRows []struct {
		ChannelNo, BindingName string
		UpdatedAt              time.Time
	}
	channelNos := make([]string, 0, len(channels))
	for _, channel := range channels {
		channelNos = append(channelNos, channel.ChannelNo)
	}
	if err := s.db.Model("plugin_idc_resource_channel_secret_binding").Safe().Ctx(ctx).
		Fields("channel_no,binding_name,updated_at").
		Where("tenant_id", idccap.DefaultTenantID).Where("status_code", "active").
		WhereIn("channel_no", channelNos).OrderAsc("channel_no").OrderAsc("binding_name").
		Limit(len(channels)*resourcecap.MaximumSecretBindingCount + 1).Scan(&activeRows); err != nil ||
		len(activeRows) > len(channels)*resourcecap.MaximumSecretBindingCount {
		return nil, bizerr.WrapCode(errOrLimit(err), CodeUnavailable)
	}
	active := make(map[string]map[string]time.Time, len(channels))
	for _, row := range activeRows {
		if active[row.ChannelNo] == nil {
			active[row.ChannelNo] = make(map[string]time.Time)
		}
		active[row.ChannelNo][row.BindingName] = row.UpdatedAt.UTC()
	}
	if s.extensions == nil {
		return result, nil
	}
	inputs := make([]plugincap.BusinessExtensionProviderExactInput, 0, len(channels))
	channelOrder := make([]string, 0, len(channels))
	for _, row := range channels {
		if row.ProviderRuntimeType != "dynamic" {
			continue
		}
		inputs = append(inputs, plugincap.BusinessExtensionProviderExactInput{
			PluginID: row.ProviderPluginID, Point: resourcecap.DynamicProviderPoint,
			ContractVersion: row.ResourceContractVersion, ReleaseVersion: row.ResourceReleaseVersion,
			ReleaseDigest: row.ResourceReleaseDigest, DeclarationDigest: row.ResourceDeclarationDigest,
		})
		channelOrder = append(channelOrder, row.ChannelNo)
	}
	if len(inputs) == 0 {
		return result, nil
	}
	evidence, err := s.extensions.GetProvidersExact(ctx, inputs)
	if err != nil || len(evidence) != len(inputs) {
		// A channel may retain a historical release that is no longer installed
		// or whose evidence was never persisted in the development database.
		// Keep the configuration row visible and mark its dynamic fields as
		// unavailable instead of making the whole provider list fail.
		logger.Warning(ctx, "provider channel historical extension evidence unavailable", "error", err)
		return result, nil
	}
	for index, item := range evidence {
		if item == nil || item.InterfaceConfiguration == nil {
			// Exact discovery keeps missing releases as nil slots. An unavailable
			// row must not discard the independently verified neighboring rows.
			continue
		}
		statuses := make([]SecretFieldStatus, 0)
		for _, field := range item.InterfaceConfiguration.Fields {
			if field == nil || field.Type != capmodel.ConfigurationFieldSecret {
				continue
			}
			updatedAt, ready := active[channelOrder[index]][field.Key]
			statuses = append(statuses, SecretFieldStatus{Key: field.Key, Required: field.Required, Ready: ready, UpdatedAt: updatedAt})
		}
		result[channelOrder[index]] = channelConfigurationStatus{SecretFields: statuses, InterfaceConfiguration: item.InterfaceConfiguration}
	}
	return result, nil
}

func providerCandidatesFromEvidence(items []*capmodel.BusinessExtensionProviderEvidence) []ProviderCandidate {
	result := make([]ProviderCandidate, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		providerID, ok := dynamicProviderID(item)
		if !ok || item.PluginID == "" || item.ReleaseVersion == "" ||
			resourcecap.ValidateProviderMetadata(item.ProviderMetadata, item.InterfaceConfiguration) != nil {
			continue
		}
		key := item.PluginID + "\x00" + item.ReleaseVersion
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			continue
		}
		result = append(result, ProviderCandidate{
			PluginID: item.PluginID, ProviderID: providerID, DisplayName: displayName,
			ReleaseVersion: item.ReleaseVersion, InterfaceConfiguration: item.InterfaceConfiguration,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DisplayName != result[j].DisplayName {
			return result[i].DisplayName < result[j].DisplayName
		}
		return result[i].PluginID < result[j].PluginID
	})
	return result
}

func errOrLimit(err error) error {
	if err != nil {
		return err
	}
	return gerror.New("Provider configuration result exceeds limit")
}

func (s *service) CreateChannel(ctx context.Context, input ChannelInput) (*Channel, error) {
	actor, err := s.authorize(ctx, PermissionWrite, "new-provider-channel")
	if err != nil {
		return nil, err
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	input.ProviderPluginID = strings.TrimSpace(input.ProviderPluginID)
	fingerprint, err := resolveCertificateFingerprint(input.TLSCertificateSHA256, "", false)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	input.TLSCertificateSHA256 = &fingerprint
	if resourcecap.ValidatePluginID(input.ProviderPluginID) != nil ||
		!validName(input.DisplayName) || !validEndpoint(input.Endpoint) {
		logger.Errorf(ctx, "provider channel input validation failed plugin=%q", input.ProviderPluginID)
		return nil, bizerr.NewCode(CodeInvalid)
	}
	input.ChannelNo, err = businessNo("CHN_")
	if err != nil {
		logger.Errorf(ctx, "create dynamic provider channel failed: %v", err)
		return nil, err
	}
	interfaceInstanceNo, err := interfaceInstanceNo()
	if err != nil {
		logger.Errorf(ctx, "create provider channel persistence failed: %v", err)
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	configurationRevision, err := configurationRevisionNo()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if s.provider.PluginID != "" && input.ProviderPluginID == s.provider.PluginID {
		input.NonSecretSettings, err = normalizeNonSecretSettings(input.NonSecretSettings, nil)
		if err != nil {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		err = s.createSourceChannel(ctx, actor, input, interfaceInstanceNo, configurationRevision)
	} else {
		err = s.createDynamicChannel(ctx, actor, input, interfaceInstanceNo, configurationRevision)
	}
	if err != nil {
		logger.Errorf(ctx, "provider channel creation rejected plugin=%s: %v", input.ProviderPluginID, err)
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.channel(ctx, input.ChannelNo)
}

func (s *service) createSourceChannel(
	ctx context.Context,
	actor int,
	input ChannelInput,
	interfaceInstanceNo string,
	configurationRevision string,
) error {
	return s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_resource_channel (tenant_id,channel_no,provider_id,provider_plugin_id,provider_runtime_type,display_name,endpoint,tls_certificate_sha256,non_secret_settings,status_code,created_by,updated_by) VALUES(?,?,?,?, 'source',?,?,?,CAST(? AS jsonb),'disabled',?,?)`, idccap.DefaultTenantID, input.ChannelNo, s.provider.ProviderID, s.provider.PluginID, input.DisplayName, input.Endpoint, *input.TLSCertificateSHA256, string(input.NonSecretSettings), actor, actor); insertErr != nil {
			return insertErr
		}
		if insertErr := insertInterfaceChannelBinding(
			tx, interfaceInstanceNo, input.ChannelNo, configurationRevision, nil, nil,
		); insertErr != nil {
			return insertErr
		}
		if insertErr := insertConfigurationRevision(ctx, tx, configurationRevision, interfaceInstanceNo, input.ChannelNo, input.Endpoint, input.NonSecretSettings, actor); insertErr != nil {
			return insertErr
		}
		return nil
	})
}

func (s *service) createDynamicChannel(
	ctx context.Context,
	actor int,
	input ChannelInput,
	interfaceInstanceNo string,
	configurationRevision string,
) error {
	if s.extensions == nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	page, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point:           resourcecap.DynamicProviderPoint,
		ContractVersion: resourcecap.DynamicProviderContractVersion,
		Page:            capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100},
	})
	if err != nil || page == nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	var selected *capmodel.BusinessExtensionProviderEvidence
	for _, item := range page.Items {
		if item == nil || (input.ProviderPluginID != "" && item.PluginID != input.ProviderPluginID) {
			continue
		}
		if selected != nil {
			return bizerr.NewCode(CodeInvalid)
		}
		selected = item
	}
	if selected == nil {
		logger.Errorf(ctx, "dynamic provider evidence not found plugin=%s", input.ProviderPluginID)
		return bizerr.NewCode(CodeInvalid)
	}
	if resourcecap.ValidateProviderMetadata(selected.ProviderMetadata, selected.InterfaceConfiguration) != nil {
		return gerror.New("resource provider metadata validation failed")
	}
	// Endpoint is stored in its own channel column, but it is also declared by
	// connector schemas so the form can render it. Satisfy schema validation
	// with the canonical endpoint and remove the duplicate before persisting
	// non-secret settings.
	settings := input.NonSecretSettings
	var settingValues map[string]json.RawMessage
	if len(settings) == 0 {
		settingValues = map[string]json.RawMessage{}
	} else if json.Unmarshal(settings, &settingValues) != nil || settingValues == nil {
		return bizerr.NewCode(CodeInvalid)
	}
	if interfaceConfigurationDeclares(selected.InterfaceConfiguration, "endpoint") {
		encodedEndpoint, marshalErr := json.Marshal(input.Endpoint)
		if marshalErr != nil {
			return bizerr.NewCode(CodeInvalid)
		}
		settingValues["endpoint"] = encodedEndpoint
	}
	settings, err = json.Marshal(settingValues)
	if err != nil {
		return bizerr.NewCode(CodeInvalid)
	}
	input.NonSecretSettings, err = normalizeNonSecretSettings(settings, selected.InterfaceConfiguration)
	if err != nil {
		return gerror.Wrap(err, "resource provider non-secret settings validation failed")
	}
	input.NonSecretSettings, err = resourcecap.CompileInterfaceSettings(input.NonSecretSettings, selected.ProviderMetadata)
	if err != nil {
		return gerror.Wrap(err, "resource provider authentication settings compilation failed")
	}
	providerID, ok := dynamicProviderID(selected)
	if !ok {
		return gerror.New("resource provider metadata providerId is invalid")
	}
	productEvidence, err := s.productFulfillmentEvidence(ctx, selected)
	if err != nil {
		return err
	}
	productProjection := productEvidence
	if productProjection == nil {
		productProjection = selected
	}
	return s.withDynamicAdmissions(ctx, selected, productEvidence, func(productCtx context.Context) error {
		return s.db.Transaction(productCtx, func(txCtx context.Context, tx gdb.TX) error {
			_, insertErr := tx.Exec(`INSERT INTO plugin_idc_resource_channel (tenant_id,channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,display_name,endpoint,tls_certificate_sha256,non_secret_settings,status_code,created_by,updated_by) VALUES(?,?,?,?, 'dynamic',?,?,?,?,?,?,?,CAST(? AS jsonb),'disabled',?,?)`, idccap.DefaultTenantID, input.ChannelNo, providerID, selected.PluginID, selected.ContractVersion, selected.ReleaseVersion, selected.ReleaseDigest, selected.DeclarationDigest, input.DisplayName, input.Endpoint, *input.TLSCertificateSHA256, string(input.NonSecretSettings), actor, actor)
			if insertErr != nil {
				return insertErr
			}
			if productEvidence == nil {
				if insertErr = insertInterfaceChannelBinding(tx, interfaceInstanceNo, input.ChannelNo, configurationRevision, selected, nil); insertErr != nil {
					return insertErr
				}
				if insertErr = insertConfigurationRevision(ctx, tx, configurationRevision, interfaceInstanceNo, input.ChannelNo, input.Endpoint, input.NonSecretSettings, actor); insertErr != nil {
					return insertErr
				}
				return nil
			}
			projection := resourceinterfaceprojectioncap.Projection{
				InterfaceInstanceNo:            interfaceInstanceNo,
				DisplayName:                    input.DisplayName,
				ActorUserID:                    actor,
				OwnerID:                        resourceinterfaceprojectioncap.OwnerID,
				ResourceChannelRef:             input.ChannelNo,
				PluginID:                       productProjection.PluginID,
				PluginReleaseVersion:           productProjection.ReleaseVersion,
				ContractVersion:                productProjection.ContractVersion,
				AuthorizedMethods:              append([]string(nil), productProjection.AuthorizedMethods...),
				ReleaseDigest:                  selected.ReleaseDigest,
				DeclarationDigest:              productProjection.DeclarationDigest,
				NewBusinessState:               resourceinterfaceprojectioncap.StateStopped,
				InterfaceConfigurationRevision: configurationRevision,
			}
			if insertErr = insertInterfaceChannelBinding(tx, interfaceInstanceNo, input.ChannelNo, configurationRevision, selected, &projection); insertErr != nil {
				return insertErr
			}
			if insertErr = insertConfigurationRevision(ctx, tx, configurationRevision, interfaceInstanceNo, input.ChannelNo, input.Endpoint, input.NonSecretSettings, actor); insertErr != nil {
				return insertErr
			}
			if insertErr = interfaceprojection.Enqueue(txCtx, tx, projection, 1); insertErr != nil {
				return insertErr
			}
			return nil
		})
	})
}

func interfaceConfigurationDeclares(configuration *capmodel.ConfigurationSchemaDeclaration, key string) bool {
	if configuration == nil {
		return false
	}
	for _, field := range configuration.Fields {
		if field != nil && field.Key == key {
			return true
		}
	}
	return false
}

func (s *service) withDynamicAdmissions(
	ctx context.Context,
	resourceEvidence *capmodel.BusinessExtensionProviderEvidence,
	productEvidence *capmodel.BusinessExtensionProviderEvidence,
	action func(context.Context) error,
) error {
	if productEvidence == nil {
		return action(ctx)
	}
	return s.extensions.WithNewBusinessAdmission(ctx, admissionInput(resourceEvidence),
		func(resourceCtx context.Context, resourceAdmission *capmodel.BusinessExtensionProviderEvidence) error {
			if !sameExtensionEvidence(resourceAdmission, resourceEvidence) {
				return gerror.New("dynamic resource Provider admission evidence changed")
			}
			// Both declarations are frozen in the same managed-plugin release.
			// The resource admission holds the plugin-wide lifecycle lock, so a
			// second admission for the same plugin would only self-conflict.
			if productEvidence.PluginID != resourceAdmission.PluginID ||
				productEvidence.ReleaseVersion != resourceAdmission.ReleaseVersion ||
				productEvidence.ReleaseDigest != resourceAdmission.ReleaseDigest ||
				validateProductFulfillmentEvidence(productEvidence) != nil {
				return gerror.New("dynamic Provider product admission evidence changed")
			}
			return action(resourceCtx)
		})
}

func admissionInput(evidence *capmodel.BusinessExtensionProviderEvidence) pluginhost.BusinessExtensionAdmissionInput {
	return pluginhost.BusinessExtensionAdmissionInput{
		PluginID: evidence.PluginID, Point: evidence.Point,
		ContractVersion: evidence.ContractVersion, ReleaseVersion: evidence.ReleaseVersion,
		ReleaseDigest: evidence.ReleaseDigest, DeclarationDigest: evidence.DeclarationDigest,
	}
}

func (s *service) productFulfillmentEvidence(
	ctx context.Context,
	resourceEvidence *capmodel.BusinessExtensionProviderEvidence,
) (*capmodel.BusinessExtensionProviderEvidence, error) {
	page, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point:           capmodel.BusinessExtensionProductFulfillment,
		ContractVersion: productfulfillmentcap.ProviderMetadataVersion,
		Page:            capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100},
	})
	if err != nil || page == nil || len(page.Items) > 100 {
		return nil, bizerr.WrapCode(errOrLimit(err), CodeUnavailable)
	}
	var selected *capmodel.BusinessExtensionProviderEvidence
	for _, item := range page.Items {
		if item == nil || item.PluginID != resourceEvidence.PluginID ||
			item.ReleaseVersion != resourceEvidence.ReleaseVersion {
			continue
		}
		if selected != nil {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		selected = item
	}
	if selected == nil || validateProductFulfillmentEvidence(selected) != nil {
		logger.Errorf(ctx, "product fulfillment evidence missing or invalid plugin=%s selected=%t", resourceEvidence.PluginID, selected != nil)
		// Creating an interface instance only freezes the connector release;
		// product-facing declaration validation happens when a product version
		// binds. Keep the interface usable while the owner projection converges.
		return nil, nil
	}
	return selected, nil
}

// Product fulfillment is intentionally generic. Resource channels may be
// used by server, SSL, domain, physical-goods, or any future product plugin;
// requiring the server-only configuration schema here would reject valid
// non-server connectors before a channel can be created.
func validateProductFulfillmentEvidence(evidence *capmodel.BusinessExtensionProviderEvidence) error {
	if evidence == nil || evidence.Point != capmodel.BusinessExtensionProductFulfillment ||
		evidence.ContractVersion == "" || len(evidence.AuthorizedMethods) == 0 {
		return gerror.New("product fulfillment evidence is incomplete")
	}
	return nil
}

func sameExtensionEvidence(left, right *capmodel.BusinessExtensionProviderEvidence) bool {
	return left != nil && right != nil && left.PluginID == right.PluginID &&
		left.Point == right.Point && left.ReleaseVersion == right.ReleaseVersion &&
		left.ContractVersion == right.ContractVersion &&
		left.ReleaseDigest == right.ReleaseDigest &&
		left.DeclarationDigest == right.DeclarationDigest
}

func dynamicProviderID(evidence *capmodel.BusinessExtensionProviderEvidence) (string, bool) {
	if evidence == nil || evidence.ProviderMetadata == nil ||
		evidence.ProviderMetadata.Type != resourcecap.DynamicProviderMetadataType ||
		evidence.ProviderMetadata.Version != resourcecap.DynamicProviderContractVersion {
		return "", false
	}
	value, ok := evidence.ProviderMetadata.Values["providerId"].(string)
	value = strings.TrimSpace(value)
	return value, ok && resourcecap.ValidateProviderID(value) == nil
}

func (s *service) UpdateChannel(ctx context.Context, input ChannelInput) (*Channel, error) {
	actor, err := s.authorize(ctx, PermissionWrite, strings.TrimSpace(input.ChannelNo))
	if err != nil {
		return nil, err
	}
	input.ChannelNo = strings.TrimSpace(input.ChannelNo)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	if resourcecap.ValidateChannelRef(input.ChannelNo) != nil || !validName(input.DisplayName) || !validEndpoint(input.Endpoint) || input.Version < 1 || !oneOf(input.Status, "disabled", "enabled", "maintenance", "deleted") {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	before, err := s.channel(ctx, input.ChannelNo)
	if err != nil {
		return nil, err
	}
	if input.RefreshProvider {
		return s.refreshChannelProvider(ctx, actor, before, input.Version)
	}
	fingerprint, err := resolveCertificateFingerprint(input.TLSCertificateSHA256, before.TLSCertificateSHA256, before.Endpoint != input.Endpoint)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	input.NonSecretSettings, err = s.validateUpdatedSettings(ctx, before, input.NonSecretSettings)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	interfaceSchema, err := s.frozenInterfaceConfiguration(ctx, before)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		configurationChanged := before.Endpoint != input.Endpoint || string(before.NonSecretSettings) != string(input.NonSecretSettings)
		configurationRevision := ""
		if configurationChanged {
			configurationRevision, err = configurationRevisionNo()
			if err != nil {
				return err
			}
		}
		if err = ensureRequiredChannelSecrets(ctx, tx, input, interfaceSchema); err != nil {
			return err
		}
		deleted := any(nil)
		if input.Status == "deleted" {
			deleted = gdb.Raw("CURRENT_TIMESTAMP")
		}
		res, e := tx.Exec(`UPDATE plugin_idc_resource_channel SET display_name=?,endpoint=?,tls_certificate_sha256=?,non_secret_settings=CAST(? AS jsonb),status_code=?,deleted_at=?,updated_by=?,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE tenant_id=? AND channel_no=? AND version=?`, input.DisplayName, input.Endpoint, fingerprint, string(input.NonSecretSettings), input.Status, deleted, actor, idccap.DefaultTenantID, input.ChannelNo, input.Version)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if configurationChanged {
			if revisionErr := insertConfigurationRevision(ctx, tx, configurationRevision, before.InterfaceInstanceNo, input.ChannelNo, input.Endpoint, input.NonSecretSettings, actor); revisionErr != nil {
				return revisionErr
			}
			if _, revisionErr := tx.Exec(`UPDATE plugin_idc_resource_interface_channel_binding SET current_configuration_revision=?,binding_version=binding_version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND channel_no=?`, configurationRevision, idccap.DefaultTenantID, input.ChannelNo); revisionErr != nil {
				return revisionErr
			}
		}
		if err = enqueueUpdatedChannelProjection(ctx, tx, input, actor, configurationChanged, configurationRevision); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if input.Status == "deleted" {
		// A deleted channel is intentionally absent from active configuration
		// reads. Return the committed transition instead of reloading it there.
		before.DisplayName = input.DisplayName
		before.Endpoint = input.Endpoint
		before.TLSCertificateSHA256 = fingerprint
		before.NonSecretSettings = input.NonSecretSettings
		before.Status = input.Status
		before.Version = input.Version + 1
		return before, nil
	}
	return s.channel(ctx, input.ChannelNo)
}

func ensureRequiredChannelSecrets(ctx context.Context, tx gdb.TX, input ChannelInput, schema *capmodel.ConfigurationSchemaDeclaration) error {
	if input.Status != "enabled" {
		return nil
	}
	required := make([]string, 0)
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil && field.Type == capmodel.ConfigurationFieldSecret && field.Required {
				required = append(required, field.Key)
			}
		}
	}
	ready := 0
	if len(required) > 0 {
		value, err := tx.Model("plugin_idc_resource_channel_secret_binding").Safe().Ctx(ctx).
			Where("tenant_id", idccap.DefaultTenantID).Where("channel_no", input.ChannelNo).
			Where("status_code", "active").WhereIn("binding_name", required).
			Fields("COUNT(DISTINCT binding_name)").Value()
		if err != nil {
			return err
		}
		ready = value.Int()
	}
	if ready != len(required) {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func enqueueUpdatedChannelProjection(ctx context.Context, tx gdb.TX, input ChannelInput, actor int, configurationChanged bool, revision string) error {
	projection, err := loadInterfaceProjection(tx, input.ChannelNo, actor)
	if err != nil || projection == nil {
		return err
	}
	newState := projectionState(input.Status)
	nameChanged := projection.DisplayName != input.DisplayName
	stateChanged := projection.NewBusinessState != newState
	sequence := (input.Version + 1) * 2
	if stateChanged {
		projection.NewBusinessState = newState
		if err = interfaceprojection.Enqueue(ctx, tx, *projection, sequence); err != nil {
			return err
		}
		sequence++
	}
	if configurationChanged {
		projection.InterfaceConfigurationRevision = revision
		if err = interfaceprojection.Enqueue(ctx, tx, *projection, sequence); err != nil {
			return err
		}
		sequence++
	}
	if nameChanged {
		projection.DisplayName = input.DisplayName
		if err = interfaceprojection.Enqueue(ctx, tx, *projection, sequence); err != nil {
			return err
		}
	}
	if !nameChanged && !stateChanged && !configurationChanged {
		return nil
	}
	versionIncrement := 1
	if configurationChanged {
		versionIncrement = 0
	}
	_, err = tx.Exec(`UPDATE plugin_idc_resource_interface_channel_binding SET display_name=?,new_business_state=?,binding_version=binding_version+?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND channel_no=?`, input.DisplayName, newState, versionIncrement, idccap.DefaultTenantID, input.ChannelNo)
	return err
}

func (s *service) channel(ctx context.Context, no string) (*Channel, error) {
	var row channelListRow
	err := s.db.Model("plugin_idc_resource_channel c").Safe().Ctx(ctx).
		LeftJoin("plugin_idc_resource_interface_channel_binding interface_binding",
			"interface_binding.tenant_id=c.tenant_id AND interface_binding.channel_no=c.channel_no").
		Fields("c.channel_no,interface_binding.interface_instance_no,c.provider_id,c.provider_plugin_id,c.provider_runtime_type,c.display_name,c.endpoint,c.tls_certificate_sha256,c.non_secret_settings::TEXT AS non_secret_settings,c.status_code,c.version",
			"interface_binding.resource_contract_version,interface_binding.resource_plugin_release_version AS resource_release_version,interface_binding.resource_release_digest,interface_binding.resource_declaration_digest").
		Where("c.tenant_id", idccap.DefaultTenantID).Where("c.channel_no", no).Limit(1).Scan(&row)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row.ChannelNo == "" {
		return nil, bizerr.NewCode(CodeConflict)
	}
	configurations, configurationErr := s.channelConfigurationStatuses(ctx, []channelListRow{row})
	if configurationErr != nil {
		return nil, configurationErr
	}
	configuration := configurations[row.ChannelNo]
	// Only the read-only list may degrade a missing historical declaration.
	// Mutation paths must still resolve this channel's exact frozen schema.
	if row.ProviderRuntimeType == "dynamic" && configuration.InterfaceConfiguration == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	publicSettings, projectionErr := resourcecap.ProjectPublicInterfaceSettings(json.RawMessage(row.NonSecretSettings), false)
	if projectionErr != nil {
		return nil, bizerr.WrapCode(projectionErr, CodeUnavailable)
	}
	return &Channel{ChannelNo: row.ChannelNo, InterfaceInstanceNo: row.InterfaceInstanceNo, ProviderID: row.ProviderID, ProviderPluginID: row.ProviderPluginID, DisplayName: row.DisplayName, Endpoint: row.Endpoint, TLSCertificateSHA256: row.TLSCertificateSHA256, NonSecretSettings: publicSettings, Status: row.StatusCode, SecretFields: configuration.SecretFields, InterfaceConfiguration: configuration.InterfaceConfiguration, ProviderContractVersion: row.ResourceContractVersion, ProviderReleaseVersion: row.ResourceReleaseVersion, ProviderReleaseDigest: row.ResourceReleaseDigest, ProviderDeclarationDigest: row.ResourceDeclarationDigest, Version: row.Version}, nil
}

func (s *service) CreatePool(ctx context.Context, input PoolInput) (*Pool, error) {
	actor, err := s.authorize(ctx, PermissionWrite, "new-resource-pool")
	if err != nil {
		return nil, err
	}
	input.ChannelNo = strings.TrimSpace(input.ChannelNo)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if resourcecap.ValidateChannelRef(input.ChannelNo) != nil || !validName(input.DisplayName) || !validPool(input) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	input.PoolNo, err = businessNo("POL_")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		channelExists, lookupErr := tx.GetValue(`SELECT 1 FROM plugin_idc_resource_channel
WHERE tenant_id=? AND channel_no=? AND deleted_at IS NULL FOR UPDATE`, idccap.DefaultTenantID, input.ChannelNo)
		if lookupErr != nil {
			return lookupErr
		}
		if channelExists.IsNil() {
			return bizerr.NewCode(CodeConflict)
		}
		result, insertErr := tx.Exec(`INSERT INTO plugin_idc_resource_pool
(tenant_id,pool_no,channel_no,display_name,resource_kind,status_code,routing_priority,routing_weight,selector,created_by,updated_by)
VALUES (?,?,?,?,?,'disabled',?,?,CAST(? AS jsonb),?,?)`, idccap.DefaultTenantID, input.PoolNo,
			input.ChannelNo, input.DisplayName, input.ResourceKind, input.RoutingPriority, input.RoutingWeight, "{}", actor, actor)
		if insertErr != nil {
			return insertErr
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.pool(ctx, input.PoolNo)
}

func (s *service) UpdatePool(ctx context.Context, input PoolInput) (*Pool, error) {
	actor, err := s.authorize(ctx, PermissionWrite, strings.TrimSpace(input.PoolNo))
	if err != nil {
		return nil, err
	}
	input.PoolNo = strings.TrimSpace(input.PoolNo)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.PoolNo == "" || !validName(input.DisplayName) || !validPool(input) || input.Version < 1 || !oneOf(input.Status, "disabled", "enabled", "draining", "maintenance", "deleted") {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	_, err = s.pool(ctx, input.PoolNo)
	if err != nil {
		return nil, err
	}
	deleted := any(nil)
	if input.Status == "deleted" {
		deleted = gdb.Raw("CURRENT_TIMESTAMP")
	}
	err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		res, updateErr := tx.Exec(`UPDATE plugin_idc_resource_pool SET display_name=?,resource_kind=?,status_code=?,routing_priority=?,routing_weight=?,deleted_at=?,updated_by=?,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE tenant_id=? AND pool_no=? AND version=?`, input.DisplayName, input.ResourceKind, input.Status, input.RoutingPriority, input.RoutingWeight, deleted, actor, idccap.DefaultTenantID, input.PoolNo, input.Version)
		if updateErr != nil {
			return updateErr
		}
		n, affectedErr := res.RowsAffected()
		if affectedErr != nil || n != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.pool(ctx, input.PoolNo)
}

func (s *service) pool(ctx context.Context, no string) (*Pool, error) {
	type poolRow struct {
		PoolNo, ChannelNo, DisplayName, ResourceKind, StatusCode string
		RoutingPriority, RoutingWeight                           int
		Version                                                  int64
	}
	var row poolRow
	err := s.db.Model("plugin_idc_resource_pool").Safe().Ctx(ctx).
		Fields("pool_no,channel_no,display_name,resource_kind,status_code,routing_priority,routing_weight,version").
		Where("tenant_id", idccap.DefaultTenantID).Where("pool_no", no).Limit(1).Scan(&row)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row.PoolNo == "" {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return &Pool{PoolNo: row.PoolNo, ChannelNo: row.ChannelNo, DisplayName: row.DisplayName, ResourceKind: row.ResourceKind, Status: row.StatusCode, RoutingPriority: row.RoutingPriority, RoutingWeight: row.RoutingWeight, Version: row.Version}, nil
}
func validName(v string) bool { return v != "" && len([]rune(v)) <= 128 }
func validPool(v PoolInput) bool {
	return oneOf(v.ResourceKind, "virtual_server", "bare_metal") && v.RoutingPriority >= 0 && v.RoutingPriority <= 100000 && v.RoutingWeight >= 0 && v.RoutingWeight <= 10000
}
func oneOf(v string, allowed ...string) bool {
	for _, x := range allowed {
		if v == x {
			return true
		}
	}
	return false
}
func validEndpoint(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return !strings.Contains(u.Path, "..")
}
func businessNo(prefix string) (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

// interfaceInstanceNo creates the product-facing opaque number without
// deriving it from the resource channel number or any Provider identity.
func interfaceInstanceNo() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "pfi_" + hex.EncodeToString(value), nil
}

// insertInterfaceChannelBinding freezes the one-to-one owner mapping in the
// same transaction as channel creation. Existing pfi numbers are never
// repointed to a different channel.
func insertInterfaceChannelBinding(
	tx gdb.TX,
	interfaceInstanceNo string,
	channelNo string,
	configurationRevision string,
	resourceEvidence *capmodel.BusinessExtensionProviderEvidence,
	projection *resourceinterfaceprojectioncap.Projection,
) error {
	if tx == nil || interfaceInstanceNo == "" || channelNo == "" || configurationRevision == "" {
		return gerror.New("resource interface-channel binding is invalid")
	}
	if projection == nil && resourceEvidence == nil {
		_, err := tx.Exec(`
INSERT INTO plugin_idc_resource_interface_channel_binding
    (tenant_id,interface_instance_no,channel_no,binding_version,current_configuration_revision)
VALUES (?,?,?,1,?)`, idccap.DefaultTenantID, interfaceInstanceNo, channelNo, configurationRevision)
		return err
	}
	if projection == nil && resourceEvidence != nil {
		methods := strings.Join(resourceEvidence.AuthorizedMethods, ",")
		_, err := tx.Exec(`
INSERT INTO plugin_idc_resource_interface_channel_binding
 (tenant_id,interface_instance_no,channel_no,binding_version,resource_plugin_id,
  resource_plugin_release_version,resource_contract_version,resource_authorized_methods,
  resource_release_digest,resource_declaration_digest,current_configuration_revision)
VALUES (?,?,?,1,?,?,?,string_to_array(?, ','),?,?,?)`, idccap.DefaultTenantID,
			interfaceInstanceNo, channelNo, resourceEvidence.PluginID,
			resourceEvidence.ReleaseVersion, resourceEvidence.ContractVersion, methods,
			resourceEvidence.ReleaseDigest, resourceEvidence.DeclarationDigest, configurationRevision)
		return err
	}
	if projection == nil || resourceEvidence == nil ||
		projection.InterfaceInstanceNo != interfaceInstanceNo ||
		projection.ResourceChannelRef != channelNo ||
		projection.InterfaceConfigurationRevision != configurationRevision ||
		resourceinterfaceprojectioncap.ValidateProjection(*projection) != nil ||
		resourceEvidence.PluginID != projection.PluginID ||
		resourceEvidence.ReleaseVersion != projection.PluginReleaseVersion ||
		resourceEvidence.ReleaseDigest != projection.ReleaseDigest {
		return gerror.New("resource interface projection binding is invalid")
	}
	if len(resourceEvidence.AuthorizedMethods) == 0 {
		return gerror.New("resource Provider methods are required")
	}
	for _, method := range resourceEvidence.AuthorizedMethods {
		if strings.TrimSpace(method) == "" || strings.Contains(method, ",") {
			return gerror.New("resource Provider method is invalid")
		}
	}
	resourceMethods := strings.Join(resourceEvidence.AuthorizedMethods, ",")
	productMethods := strings.Join(projection.AuthorizedMethods, ",")
	_, err := tx.Exec(`
INSERT INTO plugin_idc_resource_interface_channel_binding
    (tenant_id,interface_instance_no,channel_no,binding_version,display_name,
     resource_plugin_id,resource_plugin_release_version,resource_contract_version,
     resource_authorized_methods,resource_release_digest,resource_declaration_digest,
     product_plugin_id,product_plugin_release_version,product_contract_version,
     product_authorized_methods,product_release_digest,product_declaration_digest,
     new_business_state,current_configuration_revision)
VALUES (?,?,?,1,?,?,?,?,string_to_array(?, ','),?,?,?,
        ?,?,string_to_array(?, ','),?,?,?,?)`,
		idccap.DefaultTenantID, interfaceInstanceNo, channelNo, projection.DisplayName,
		resourceEvidence.PluginID, resourceEvidence.ReleaseVersion,
		resourceEvidence.ContractVersion, resourceMethods, resourceEvidence.ReleaseDigest,
		resourceEvidence.DeclarationDigest, projection.PluginID,
		projection.PluginReleaseVersion, projection.ContractVersion, productMethods,
		projection.ReleaseDigest, projection.DeclarationDigest, projection.NewBusinessState,
		configurationRevision)
	return err
}

func loadInterfaceProjection(
	tx gdb.TX,
	channelNo string,
	actor int,
) (*resourceinterfaceprojectioncap.Projection, error) {
	type row struct {
		InterfaceInstanceNo            string   `orm:"interface_instance_no"`
		DisplayName                    string   `orm:"display_name"`
		PluginID                       string   `orm:"plugin_id"`
		PluginReleaseVersion           string   `orm:"plugin_release_version"`
		ContractVersion                string   `orm:"contract_version"`
		AuthorizedMethods              []string `orm:"authorized_methods"`
		ReleaseDigest                  string   `orm:"release_digest"`
		DeclarationDigest              string   `orm:"declaration_digest"`
		NewBusinessState               string   `orm:"new_business_state"`
		InterfaceConfigurationRevision string   `orm:"current_configuration_revision"`
	}
	var value row
	if err := tx.GetScan(&value, `
SELECT interface_instance_no,display_name,
       product_plugin_id AS plugin_id,
       product_plugin_release_version AS plugin_release_version,
       product_contract_version AS contract_version,
       product_authorized_methods AS authorized_methods,
       product_release_digest AS release_digest,
       product_declaration_digest AS declaration_digest,
       new_business_state,current_configuration_revision
FROM plugin_idc_resource_interface_channel_binding
WHERE tenant_id=? AND channel_no=? FOR UPDATE`, idccap.DefaultTenantID, channelNo); err != nil {
		return nil, err
	}
	if value.PluginID == "" {
		return nil, nil
	}
	projection := &resourceinterfaceprojectioncap.Projection{
		InterfaceInstanceNo:            value.InterfaceInstanceNo,
		DisplayName:                    value.DisplayName,
		ActorUserID:                    actor,
		OwnerID:                        resourceinterfaceprojectioncap.OwnerID,
		ResourceChannelRef:             channelNo,
		PluginID:                       value.PluginID,
		PluginReleaseVersion:           value.PluginReleaseVersion,
		ContractVersion:                value.ContractVersion,
		AuthorizedMethods:              append([]string(nil), value.AuthorizedMethods...),
		ReleaseDigest:                  value.ReleaseDigest,
		DeclarationDigest:              value.DeclarationDigest,
		NewBusinessState:               value.NewBusinessState,
		InterfaceConfigurationRevision: value.InterfaceConfigurationRevision,
	}
	if err := resourceinterfaceprojectioncap.ValidateProjection(*projection); err != nil {
		return nil, gerror.Wrap(err, "load resource interface projection")
	}
	return projection, nil
}

func configurationRevisionNo() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "rci_" + hex.EncodeToString(value), nil
}

func insertConfigurationRevision(
	ctx context.Context,
	tx gdb.TX,
	revisionNo, interfaceNo, channelNo, endpoint string,
	settings json.RawMessage,
	actor int,
) error {
	var rows []struct {
		BindingName string `orm:"binding_name"`
		SecretRef   string `orm:"secret_ref"`
	}
	columns := dao.ChannelSecretBinding.Columns()
	if err := tx.Model(dao.ChannelSecretBinding.Table()).Safe().Ctx(ctx).
		Fields(columns.BindingName, columns.SecretRef).
		Where(columns.TenantId, idccap.DefaultTenantID).
		Where(columns.ChannelNo, channelNo).
		Where(columns.StatusCode, "active").
		OrderAsc(columns.BindingName).
		Limit(resourcecap.MaximumSecretBindingCount + 1).
		Scan(&rows); err != nil {
		return err
	}
	if len(rows) > resourcecap.MaximumSecretBindingCount {
		return gerror.New("resource channel secret bindings exceed the limit")
	}
	bindings := make(map[string]string, len(rows))
	for _, row := range rows {
		bindings[row.BindingName] = row.SecretRef
	}
	encodedBindings, err := json.Marshal(bindings)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
INSERT INTO plugin_idc_resource_interface_configuration_revision
 (tenant_id,revision_no,interface_instance_no,channel_no,endpoint,
  non_secret_settings,secret_bindings,created_by)
VALUES (?,?,?,?,?,CAST(? AS jsonb),CAST(? AS jsonb),?)
`, idccap.DefaultTenantID, revisionNo, interfaceNo, channelNo, endpoint,
		string(settings), string(encodedBindings), actor)
	return err
}

func projectionState(channelStatus string) string {
	if channelStatus == "enabled" {
		return resourceinterfaceprojectioncap.StateAccepting
	}
	return resourceinterfaceprojectioncap.StateStopped
}

func (s *service) validateUpdatedSettings(
	ctx context.Context,
	channel *Channel,
	settings json.RawMessage,
) (json.RawMessage, error) {
	if len(settings) == 0 {
		settings = channel.NonSecretSettings
	}
	evidence, err := s.frozenProviderEvidence(ctx, channel)
	if err != nil {
		return nil, err
	}
	if evidence == nil {
		return normalizeNonSecretSettings(settings, nil)
	}
	if resourcecap.ValidateProviderMetadata(evidence.ProviderMetadata, evidence.InterfaceConfiguration) != nil {
		return nil, gerror.New("resource Provider authentication declaration is invalid")
	}
	normalized, err := normalizeNonSecretSettings(settings, evidence.InterfaceConfiguration)
	if err != nil {
		return nil, err
	}
	return resourcecap.CompileInterfaceSettings(normalized, evidence.ProviderMetadata)
}

func (s *service) frozenInterfaceConfiguration(ctx context.Context, channel *Channel) (*capmodel.ConfigurationSchemaDeclaration, error) {
	evidence, err := s.frozenProviderEvidence(ctx, channel)
	if err != nil || evidence == nil {
		return nil, err
	}
	return evidence.InterfaceConfiguration, nil
}

func (s *service) frozenProviderEvidence(ctx context.Context, channel *Channel) (*capmodel.BusinessExtensionProviderEvidence, error) {
	if s.extensions == nil {
		return nil, nil
	}
	var evidenceRef struct {
		PluginID, ReleaseVersion, ContractVersion, ReleaseDigest, DeclarationDigest string
	}
	if err := s.db.GetScan(ctx, &evidenceRef, `
SELECT resource_plugin_id AS plugin_id,
       resource_plugin_release_version AS release_version,
       resource_contract_version AS contract_version,
       resource_release_digest AS release_digest,
       resource_declaration_digest AS declaration_digest
FROM plugin_idc_resource_interface_channel_binding
WHERE tenant_id=? AND channel_no=?`, idccap.DefaultTenantID, channel.ChannelNo); err != nil {
		return nil, err
	}
	if evidenceRef.PluginID == "" {
		return nil, nil
	}
	evidence, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: evidenceRef.PluginID, ReleaseVersion: evidenceRef.ReleaseVersion,
		Point: resourcecap.DynamicProviderPoint, ContractVersion: evidenceRef.ContractVersion,
		ReleaseDigest: evidenceRef.ReleaseDigest, DeclarationDigest: evidenceRef.DeclarationDigest,
	})
	if err != nil || evidence == nil {
		return nil, gerror.New("resource Provider configuration declaration is unavailable")
	}
	return evidence, nil
}

func normalizeNonSecretSettings(
	payload json.RawMessage,
	schema *capmodel.ConfigurationSchemaDeclaration,
) (json.RawMessage, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if len(payload) > 1<<16 || !json.Valid(payload) {
		return nil, gerror.New("resource Provider non-secret settings are invalid")
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(payload, &values) != nil || values == nil || len(values) > 64 {
		return nil, gerror.New("resource Provider non-secret settings are invalid")
	}
	allowed := make(map[string]*capmodel.ConfigurationFieldDeclaration)
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil {
				allowed[field.Key] = field
			}
		}
	}
	for key, value := range values {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		for _, forbidden := range []string{"password", "secret", "token", "credential", "username", "apikey", "accesskey"} {
			if strings.Contains(normalized, forbidden) {
				return nil, gerror.New("resource Provider non-secret setting contains a confidential field")
			}
		}
		if schema != nil {
			field, ok := allowed[key]
			if !ok || field.Type == capmodel.ConfigurationFieldSecret || !validDeclaredSettingValue(value, field.Type) {
				return nil, gerror.New("resource Provider non-secret setting is undeclared")
			}
		}
		if len(value) == 0 || len(value) > 1<<14 {
			return nil, gerror.New("resource Provider non-secret setting value is invalid")
		}
	}
	if schema != nil {
		for key, field := range allowed {
			if field.Required && field.Type != capmodel.ConfigurationFieldSecret {
				if _, exists := values[key]; !exists {
					return nil, gerror.New("resource Provider required non-secret setting is missing")
				}
			}
		}
	}
	canonical, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func validDeclaredSettingValue(value json.RawMessage, fieldType string) bool {
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.UseNumber()
	if decoder.Decode(&decoded) != nil {
		return false
	}
	switch fieldType {
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine,
		capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldDate,
		capmodel.ConfigurationFieldDateTime:
		_, ok := decoded.(string)
		return ok
	case capmodel.ConfigurationFieldInteger:
		number, ok := decoded.(json.Number)
		return ok && !strings.ContainsAny(number.String(), ".eE")
	case capmodel.ConfigurationFieldDecimal:
		_, ok := decoded.(json.Number)
		return ok
	case capmodel.ConfigurationFieldBoolean:
		_, ok := decoded.(bool)
		return ok
	case capmodel.ConfigurationFieldMultiSelect:
		items, ok := decoded.([]any)
		if !ok || len(items) > 256 {
			return false
		}
		for _, item := range items {
			if _, ok = item.(string); !ok {
				return false
			}
		}
		return true
	case capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress:
		text, ok := decoded.(string)
		return ok && len(text) > 0 && len(text) <= 128 &&
			!strings.ContainsAny(text, "/\\\r\n\x00")
	case capmodel.ConfigurationFieldFile:
		// A file setting is rejected until the resource owner has a host file-token resolver.
		return false
	default:
		return false
	}
}
