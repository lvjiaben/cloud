// Package manifestview implements immutable source-plugin manifest snapshots
// returned through the public pluginhost upgrade callback contract.
package manifestview

import (
	pluginv1 "lina-core/api/plugin/v1"
	"lina-core/pkg/plugin/capability/capmodel"
)

// Snapshot is the host-owned immutable manifest snapshot view.
type Snapshot struct {
	value capmodel.ManifestSnapshot
}

// NewSnapshot creates one immutable manifest snapshot view.
func NewSnapshot(value *capmodel.ManifestSnapshot) *Snapshot {
	if value == nil {
		return nil
	}
	return &Snapshot{value: cloneManifestSnapshot(*value)}
}

// ID returns the plugin identifier recorded in the manifest snapshot.
func (s *Snapshot) ID() string {
	if s == nil {
		return ""
	}
	return s.value.ID
}

// Name returns the plugin display name recorded in the manifest snapshot.
func (s *Snapshot) Name() string {
	if s == nil {
		return ""
	}
	return s.value.Name
}

// Version returns the plugin version recorded in the manifest snapshot.
func (s *Snapshot) Version() string {
	if s == nil {
		return ""
	}
	return s.value.Version
}

// Type returns the plugin type recorded in the manifest snapshot.
func (s *Snapshot) Type() pluginv1.PluginType {
	if s == nil {
		return ""
	}
	return s.value.Type
}

// Values returns a copy of the typed manifest snapshot primitive.
func (s *Snapshot) Values() *capmodel.ManifestSnapshot {
	if s == nil {
		return nil
	}
	value := cloneManifestSnapshot(s.value)
	return &value
}

func cloneManifestSnapshot(value capmodel.ManifestSnapshot) capmodel.ManifestSnapshot {
	value.BusinessExtensions = cloneBusinessExtensions(value.BusinessExtensions)
	value.AdminSurfaces = cloneFrontendSurfaces(value.AdminSurfaces)
	value.CustomerSurfaces = cloneFrontendSurfaces(value.CustomerSurfaces)
	return value
}

func cloneBusinessExtensions(items []*capmodel.BusinessExtensionDeclaration) []*capmodel.BusinessExtensionDeclaration {
	result := make([]*capmodel.BusinessExtensionDeclaration, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.Methods = append([]string(nil), item.Methods...)
		next.InterfaceConfiguration = cloneConfiguration(item.InterfaceConfiguration)
		next.ProductConfiguration = cloneConfiguration(item.ProductConfiguration)
		next.CustomerConfiguration = cloneConfiguration(item.CustomerConfiguration)
		next.ProviderMetadata = cloneProviderMetadata(item.ProviderMetadata)
		result = append(result, &next)
	}
	return result
}

func cloneProviderMetadata(item *capmodel.BusinessExtensionProviderMetadata) *capmodel.BusinessExtensionProviderMetadata {
	if item == nil {
		return nil
	}
	next := *item
	next.Values = cloneOpaqueValues(item.Values)
	return &next
}

func cloneOpaqueValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = cloneOpaqueValue(value)
	}
	return result
}

func cloneOpaqueValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneOpaqueValues(typed)
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = cloneOpaqueValue(typed[index])
		}
		return result
	default:
		return typed
	}
}

func cloneConfiguration(item *capmodel.ConfigurationSchemaDeclaration) *capmodel.ConfigurationSchemaDeclaration {
	if item == nil {
		return nil
	}
	next := *item
	next.Fields = make([]*capmodel.ConfigurationFieldDeclaration, 0, len(item.Fields))
	for _, field := range item.Fields {
		if field == nil {
			next.Fields = append(next.Fields, nil)
			continue
		}
		cloned := *field
		cloned.Label = cloneLocalizedText(field.Label)
		cloned.Description = cloneLocalizedText(field.Description)
		cloned.StaticCandidates = make([]*capmodel.ConfigurationCandidate, 0, len(field.StaticCandidates))
		for _, candidate := range field.StaticCandidates {
			if candidate == nil {
				cloned.StaticCandidates = append(cloned.StaticCandidates, nil)
				continue
			}
			candidateClone := *candidate
			candidateClone.Label = cloneLocalizedText(candidate.Label)
			cloned.StaticCandidates = append(cloned.StaticCandidates, &candidateClone)
		}
		if field.DynamicCandidates != nil {
			dynamic := *field.DynamicCandidates
			cloned.DynamicCandidates = &dynamic
		}
		next.Fields = append(next.Fields, &cloned)
	}
	next.Conditions = make([]*capmodel.ConfigurationRuleDeclaration, 0, len(item.Conditions))
	for _, rule := range item.Conditions {
		if rule == nil {
			next.Conditions = append(next.Conditions, nil)
			continue
		}
		cloned := *rule
		cloned.When = cloneConfigurationCondition(rule.When)
		cloned.Actions = make([]*capmodel.ConfigurationActionDeclaration, 0, len(rule.Actions))
		for _, action := range rule.Actions {
			if action == nil {
				cloned.Actions = append(cloned.Actions, nil)
				continue
			}
			actionClone := *action
			actionClone.Values = append([]string(nil), action.Values...)
			cloned.Actions = append(cloned.Actions, &actionClone)
		}
		next.Conditions = append(next.Conditions, &cloned)
	}
	next.UploadPolicies = make([]*capmodel.ConfigurationUploadPolicyDeclaration, 0, len(item.UploadPolicies))
	for _, policy := range item.UploadPolicies {
		if policy == nil {
			next.UploadPolicies = append(next.UploadPolicies, nil)
			continue
		}
		cloned := *policy
		cloned.AllowedMIMETypes = append([]string(nil), policy.AllowedMIMETypes...)
		cloned.AllowedExtensions = append([]string(nil), policy.AllowedExtensions...)
		next.UploadPolicies = append(next.UploadPolicies, &cloned)
	}
	return &next
}

func cloneConfigurationCondition(item *capmodel.ConfigurationConditionDeclaration) *capmodel.ConfigurationConditionDeclaration {
	if item == nil {
		return nil
	}
	next := *item
	next.Children = make([]*capmodel.ConfigurationConditionDeclaration, 0, len(item.Children))
	for _, child := range item.Children {
		next.Children = append(next.Children, cloneConfigurationCondition(child))
	}
	return &next
}

func cloneLocalizedText(item *capmodel.LocalizedText) *capmodel.LocalizedText {
	if item == nil {
		return nil
	}
	next := *item
	return &next
}

func cloneFrontendSurfaces(items []*capmodel.FrontendSurfaceDeclaration) []*capmodel.FrontendSurfaceDeclaration {
	result := make([]*capmodel.FrontendSurfaceDeclaration, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.BridgeCapabilities = append([]string(nil), item.BridgeCapabilities...)
		result = append(result, &next)
	}
	return result
}
