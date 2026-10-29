// Merchant choice restrictions are frozen with commercial presentation. They
// reuse the existing bounded rule engine and can only narrow candidate sets;
// provider declarations and historical configuration digests stay unchanged.
package configurationcontract

import (
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
)

// WithPresentationChoiceRules returns an immutable evaluation view with local
// choice restrictions appended to provider rules. The original declaration is
// retained for evidence/digests. Rules may only filter the owning choice field,
// so presentation can never relax a provider's required or secret controls.
func WithPresentationChoiceRules(schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]Presentation) (*capmodel.ConfigurationSchemaDeclaration, error) {
	var (
		fields = make(map[string]*capmodel.ConfigurationFieldDeclaration)
		keys   = make([]string, 0, len(presentations))
	)
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil {
				fields[field.Key] = field
			}
		}
	}
	for key, presentation := range presentations {
		if len(presentation.ChoiceRules) > 0 {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return schema, nil
	}
	if schema == nil {
		return nil, gerror.New("choice rules require a customer declaration")
	}
	sort.Strings(keys)
	result := *schema
	result.Conditions = append([]*capmodel.ConfigurationRuleDeclaration(nil), schema.Conditions...)
	for _, key := range keys {
		field := fields[key]
		if field == nil || field.Type != capmodel.ConfigurationFieldSingleSelect {
			return nil, gerror.New("choice rules require an enabled select field")
		}
		for _, rule := range presentations[key].ChoiceRules {
			if err := validateConfigurationRule(rule, fields); err != nil {
				return nil, err
			}
			for _, action := range rule.Actions {
				if action.Field != key || action.Action != capmodel.ConfigurationActionFilterCandidates {
					return nil, gerror.New("commercial choice rules may only narrow their own choices")
				}
			}
			result.Conditions = append(result.Conditions, rule)
			if len(result.Conditions) > maximumConfigurationRules {
				return nil, gerror.New("commercial choice rules exceed their bound")
			}
		}
	}
	return &result, nil
}
