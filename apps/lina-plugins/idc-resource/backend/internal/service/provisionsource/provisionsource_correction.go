// This file resolves operational corrections inside the resource owner. It
// preserves every purchased size/value, exposes labels only and never writes
// to an upstream while preparing or validating an administrator selection.
package provisionsource

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

const maximumRecoveryInterfaces = 100
const recoveryProviderID = "zjmfcloud-v1"

var recoveryInterfacePattern = regexp.MustCompile(`^pfi_[0-9a-f]{32}$`)

// recoveryBinding is a secret-free local projection of one enabled interface.
type recoveryBinding struct{ InterfaceInstanceNo, ConfigurationRevision, ChannelNo, ProviderID, DisplayName string }

// InspectRecovery returns the original specification and bounded compatible
// upstream choices. Existing customer accounts and instances are never changed.
func (s *Service) InspectRecovery(ctx context.Context, input provisionresourcecap.RecoverySelectionRequest) (*provisionresourcecap.RecoveryEditor, error) {
	if s == nil || s.database == nil || s.candidates == nil || provisionresourcecap.ValidateTaskRequest(input.Request) != nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	original, err := originalRecoverySelection(input.Request)
	if err != nil {
		return nil, err
	}
	origin, err := s.recoveryBinding(ctx, input.Request.InterfaceInstanceNo, false)
	if err != nil || origin.ProviderID != recoveryProviderID {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	selection := original
	if input.Selection != nil {
		selection = *input.Selection
	}
	if selection.InterfaceInstanceNo == "" {
		selection.InterfaceInstanceNo = original.InterfaceInstanceNo
	}
	var bindings []recoveryBinding
	err = s.database.Model(do.Channel{}).Ctx(ctx).As("channel").
		InnerJoin("plugin_idc_resource_interface_channel_binding binding", "channel.tenant_id=binding.tenant_id AND channel.channel_no=binding.channel_no").
		Where("binding.tenant_id", 0).Where("channel.status_code", "enabled").Where("channel.provider_id", origin.ProviderID).
		Fields("binding.interface_instance_no,binding.current_configuration_revision AS configuration_revision,channel.channel_no,channel.provider_id,channel.display_name").
		OrderAsc("channel.display_name").OrderAsc("binding.interface_instance_no").Limit(maximumRecoveryInterfaces + 1).Scan(&bindings)
	if err != nil || len(bindings) > maximumRecoveryInterfaces {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	editor := &provisionresourcecap.RecoveryEditor{Original: original, Selection: selection, Interfaces: []provisionresourcecap.RecoveryOption{}, Regions: []provisionresourcecap.RecoveryOption{}, Lines: []provisionresourcecap.RecoveryOption{}, OperatingSystems: []provisionresourcecap.RecoveryOption{}}
	for _, binding := range bindings {
		editor.Interfaces = append(editor.Interfaces, provisionresourcecap.RecoveryOption{Value: binding.InterfaceInstanceNo, LabelZhCN: binding.DisplayName, LabelEnUS: binding.DisplayName})
	}
	editor.Specifications, err = recoverySpecifications(input.Request)
	if err != nil {
		return nil, err
	}
	editor.Original.InterfaceLabel = origin.DisplayName
	selected, err := s.recoveryBinding(ctx, selection.InterfaceInstanceNo, true)
	if err != nil || selected.ProviderID != origin.ProviderID {
		if input.Selection == nil {
			// An inactive original channel still permits the administrator to
			// inspect original terms and explicitly choose another upstream.
			editor.Selection = provisionresourcecap.RecoveryConfiguration{}
			return editor, nil
		}
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	editor.Selection.InterfaceConfigurationRevision = selected.ConfigurationRevision
	payload, err := recoveryCandidatePayload(selection.Region)
	if err != nil {
		return nil, err
	}
	candidates, err := s.candidates.ResolveCandidates(ctx, resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: selection.InterfaceInstanceNo, ExpectedConfigurationRevision: selected.ConfigurationRevision, Method: resourcecap.DynamicProviderMethodConfigurationCandidates, Values: payload})
	if err != nil || candidates == nil {
		// A read outage must not hide compatible replacement channels or the
		// purchased terms. Empty option sets still prevent ValidateRecovery
		// from accepting an unverified node or image.
		editor.Selection.Region = ""
		editor.Selection.Line = ""
		editor.Selection.OperatingSystem = ""
		editor.Selection.InterfaceLabel = selected.DisplayName
		return editor, nil
	}
	for _, field := range candidates.Fields {
		options := make([]provisionresourcecap.RecoveryOption, 0, len(field.Items))
		for _, item := range field.Items {
			options = append(options, provisionresourcecap.RecoveryOption{Value: item.Value, LabelZhCN: item.LabelZhCN, LabelEnUS: item.LabelEnUS})
		}
		switch field.FieldKey {
		case string(resourcecap.ServerProvisionComponentRegion):
			editor.Regions = options
		case string(resourcecap.ServerProvisionComponentLine):
			editor.Lines = options
		case string(resourcecap.ServerProvisionComponentOperatingSystem):
			editor.OperatingSystems = options
		}
	}
	if !recoveryOptionExists(editor.Regions, editor.Selection.Region) {
		editor.Selection.Region = ""
	}
	if !recoveryOptionExists(editor.Lines, editor.Selection.Line) {
		editor.Selection.Line = ""
	}
	if !recoveryOptionExists(editor.OperatingSystems, editor.Selection.OperatingSystem) {
		editor.Selection.OperatingSystem = ""
	}
	if editor.Selection.InterfaceInstanceNo == original.InterfaceInstanceNo {
		editor.Original.RegionLabel = recoveryOptionLabel(editor.Regions, original.Region)
		editor.Original.LineLabel = recoveryOptionLabel(editor.Lines, original.Line)
		editor.Original.OperatingSystemLabel = recoveryOptionLabel(editor.OperatingSystems, original.OperatingSystem)
	}
	editor.Original.InterfaceLabel = origin.DisplayName
	editor.Selection.InterfaceLabel = selected.DisplayName
	editor.Selection.RegionLabel = recoveryOptionLabel(editor.Regions, selection.Region)
	editor.Selection.LineLabel = recoveryOptionLabel(editor.Lines, selection.Line)
	editor.Selection.OperatingSystemLabel = recoveryOptionLabel(editor.OperatingSystems, selection.OperatingSystem)
	return editor, nil
}

// ValidateRecovery pins the current connection and verifies every chosen ID
// against the exact upstream option set before core records its command.
func (s *Service) ValidateRecovery(ctx context.Context, input provisionresourcecap.RecoverySelectionRequest) (*provisionresourcecap.RecoveryConfiguration, error) {
	if input.Selection == nil {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	editor, err := s.InspectRecovery(ctx, input)
	if err != nil {
		return nil, err
	}
	if input.Selection.InterfaceConfigurationRevision != editor.Selection.InterfaceConfigurationRevision ||
		!recoveryOptionExists(editor.Regions, editor.Selection.Region) || !recoveryOptionExists(editor.Lines, editor.Selection.Line) || !recoveryOptionExists(editor.OperatingSystems, editor.Selection.OperatingSystem) {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	if _, err = s.correctedRequest(ctx, input.Request, editor.Selection); err != nil {
		return nil, err
	}
	return &editor.Selection, nil
}

// recoveryBinding reads one operational channel without exposing its endpoint,
// authentication data or configuration to the fulfillment owner or browser.
func (s *Service) recoveryBinding(ctx context.Context, interfaceNo string, enabled bool) (*recoveryBinding, error) {
	if !recoveryInterfacePattern.MatchString(interfaceNo) {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	var binding *recoveryBinding
	model := s.database.Model(do.Channel{}).Ctx(ctx).As("channel").
		InnerJoin("plugin_idc_resource_interface_channel_binding binding", "channel.tenant_id=binding.tenant_id AND channel.channel_no=binding.channel_no").
		Where("binding.tenant_id", 0).Where("binding.interface_instance_no", interfaceNo).
		Where("channel.provider_runtime_type", "dynamic")
	if enabled {
		model = model.Where("channel.status_code", "enabled")
	}
	err := model.Fields("binding.interface_instance_no,binding.current_configuration_revision AS configuration_revision,channel.channel_no,channel.provider_id,channel.display_name").Limit(1).Scan(&binding)
	if err != nil || binding == nil {
		return nil, provisionresourcecap.ErrParametersUnavailable
	}
	return binding, nil
}

// correctedRequest copies original documents and changes only operational IDs.
// All quantities, quotas, prices and paid identities remain byte-for-byte facts.
func (s *Service) correctedRequest(ctx context.Context, request provisionresourcecap.TaskRequest, correction provisionresourcecap.RecoveryConfiguration) (provisionresourcecap.TaskRequest, error) {
	original, err := s.recoveryBinding(ctx, request.InterfaceInstanceNo, false)
	if err != nil {
		return request, err
	}
	selected, err := s.recoveryBinding(ctx, correction.InterfaceInstanceNo, true)
	if err != nil || original.ProviderID != recoveryProviderID || original.ProviderID != selected.ProviderID || selected.ConfigurationRevision != correction.InterfaceConfigurationRevision || correction.Region == "" || correction.Line == "" || correction.OperatingSystem == "" {
		return request, provisionresourcecap.ErrParametersUnavailable
	}
	var product, customer map[string]json.RawMessage
	if json.Unmarshal(request.ProductConfiguration, &product) != nil || json.Unmarshal(request.CustomerConfiguration, &customer) != nil || product == nil || customer == nil {
		return request, provisionresourcecap.ErrParametersUnavailable
	}
	// JSON marshal of strings is infallible, but every error is still handled.
	for _, field := range []struct {
		values     map[string]json.RawMessage
		key, value string
	}{
		{product, string(resourcecap.ServerProvisionComponentRegion), correction.Region}, {product, string(resourcecap.ServerProvisionComponentLine), correction.Line}, {customer, string(resourcecap.ServerProvisionComponentOperatingSystem), correction.OperatingSystem},
	} {
		encoded, encodeErr := json.Marshal(field.value)
		if encodeErr != nil {
			return request, encodeErr
		}
		field.values[field.key] = encoded
	}
	if err = applyRecoveryPasswordRules(product, correction); err != nil {
		return request, err
	}
	request.ProductConfiguration, err = json.Marshal(product)
	if err != nil {
		return request, err
	}
	request.CustomerConfiguration, err = json.Marshal(customer)
	if err != nil {
		return request, err
	}
	request.InterfaceInstanceNo, request.InterfaceConfigurationRevision = correction.InterfaceInstanceNo, correction.InterfaceConfigurationRevision
	return request, nil
}

// originalRecoverySelection reads only native routing values from safe frozen JSON.
func originalRecoverySelection(request provisionresourcecap.TaskRequest) (provisionresourcecap.RecoveryConfiguration, error) {
	var product struct {
		Region string `json:"region"`
		Line   string `json:"line"`
	}
	var customer struct {
		OperatingSystem string `json:"operating_system"`
	}
	if json.Unmarshal(request.ProductConfiguration, &product) != nil || json.Unmarshal(request.CustomerConfiguration, &customer) != nil {
		return provisionresourcecap.RecoveryConfiguration{}, provisionresourcecap.ErrParametersUnavailable
	}
	return provisionresourcecap.RecoveryConfiguration{InterfaceInstanceNo: request.InterfaceInstanceNo, InterfaceConfigurationRevision: request.InterfaceConfigurationRevision, Region: product.Region, Line: product.Line, OperatingSystem: customer.OperatingSystem}, nil
}

// recoveryOptionExists checks exact membership without accepting a typed-in ID.
func recoveryOptionExists(options []provisionresourcecap.RecoveryOption, value string) bool {
	for _, option := range options {
		if option.Value == value && value != "" {
			return true
		}
	}
	return false
}

// recoveryCandidatePayload uses the connector's existing bounded read protocol.
func recoveryCandidatePayload(region string) (json.RawMessage, error) {
	type query struct {
		FieldKey     string   `json:"fieldKey"`
		ParentValues []string `json:"parentValues,omitempty"`
	}
	parent := []string{}
	if region != "" {
		parent = append(parent, region)
	}
	return json.Marshal(struct {
		SchemaVersion string  `json:"schemaVersion"`
		Requests      []query `json:"requests"`
	}{"v1", []query{{FieldKey: string(resourcecap.ServerProvisionComponentRegion)}, {FieldKey: string(resourcecap.ServerProvisionComponentLine), ParentValues: parent}, {FieldKey: string(resourcecap.ServerProvisionComponentOperatingSystem)}}})
}

// recoverySpecifications reports typed original sizes; routing/image fields are
// separately editable and therefore excluded from this read-only summary.
func recoverySpecifications(request provisionresourcecap.TaskRequest) ([]provisionresourcecap.RecoverySpecification, error) {
	components, err := parseFrozenServerConfiguration(request.ProductConfiguration, request.CustomerConfiguration)
	if err != nil {
		return nil, err
	}
	result := []provisionresourcecap.RecoverySpecification{}
	for _, component := range components {
		var value int64
		switch component.Type {
		case resourcecap.ServerProvisionComponentCPU:
			value = component.Cores
		case resourcecap.ServerProvisionComponentMemory:
			value = component.Megabytes
		case resourcecap.ServerProvisionComponentSystemDisk, resourcecap.ServerProvisionComponentDataDisk:
			value = component.Gigabytes
		case resourcecap.ServerProvisionComponentBandwidth:
			value = component.MegabitsPerSecond
		default:
			continue
		}
		result = append(result, provisionresourcecap.RecoverySpecification{Key: string(component.Type), Value: strconv.FormatInt(value, 10)})
	}
	return result, nil
}

// taskOwnerMetadataWithRevision retains the original request digest while
// pinning the corrected operational connection for the resource executor.
func taskOwnerMetadataWithRevision(request provisionresourcecap.TaskRequest, revision, evidenceNo, evidenceDigest string, correction *provisionresourcecap.RecoveryConfiguration) (json.RawMessage, error) {
	raw, err := taskOwnerMetadata(request, evidenceNo, evidenceDigest)
	if err != nil {
		return nil, err
	}
	facts, err := decodeTaskOwnerFacts(string(raw))
	if err != nil {
		return nil, err
	}
	facts.InterfaceConfigurationRevision = revision
	facts.RecoveryConfiguration = correction
	return json.Marshal(facts)
}

// recoveryOptionLabel freezes an actual safe upstream label for operator history.
func recoveryOptionLabel(options []provisionresourcecap.RecoveryOption, value string) string {
	for _, option := range options {
		if option.Value == value {
			return option.LabelZhCN
		}
	}
	return ""
}

// sameTaskCorrection prevents an idempotent successor from being silently
// rebound to different routing values after its first durable creation.
func sameTaskCorrection(row *taskRow, correction *provisionresourcecap.RecoveryConfiguration) bool {
	if row == nil {
		return false
	}
	facts, err := decodeTaskOwnerFacts(row.OwnerMetadata)
	if err != nil {
		return false
	}
	if facts.RecoveryConfiguration == nil || correction == nil {
		return facts.RecoveryConfiguration == nil && correction == nil
	}
	return *facts.RecoveryConfiguration == *correction
}

// applyRecoveryPasswordRules changes only nonsecret generation settings. Zero
// means retain the paid rules; no plaintext password is accepted or returned.
func applyRecoveryPasswordRules(product map[string]json.RawMessage, correction provisionresourcecap.RecoveryConfiguration) error {
	if correction.PasswordLength == 0 {
		if correction.PasswordCharacters != "" {
			return provisionresourcecap.ErrParametersUnavailable
		}
		return nil
	}
	length := correction.PasswordLength
	policy := resourcecap.ServerGenerationPolicy{PasswordLength: &length, PasswordCharacters: strings.Split(correction.PasswordCharacters, ",")}
	payload, err := json.Marshal(policy)
	if err != nil {
		return provisionresourcecap.ErrParametersUnavailable
	}
	if _, err = resourcecap.ParseServerGenerationPolicy(payload); err != nil {
		return provisionresourcecap.ErrParametersUnavailable
	}
	encodedLength, err := json.Marshal(length)
	if err != nil {
		return provisionresourcecap.ErrParametersUnavailable
	}
	encodedCharacters, err := json.Marshal(policy.PasswordCharacters)
	if err != nil {
		return provisionresourcecap.ErrParametersUnavailable
	}
	product["password_length"] = encodedLength
	product["password_characters"] = encodedCharacters
	return nil
}
