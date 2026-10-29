// Saved values may come from an intentionally partial import. Reusing their
// digest must not turn missing required inputs into a complete configuration.
package configurationcontract

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
)

// ValidateStoredCompleteness applies declaration presence/relationship rules to
// an already typed, digest-verified stored JSON document. It neither invents
// dynamic candidates nor attests current supplier eligibility. Full compilation
// and supplier admission remain required for new or changed values.
func ValidateStoredCompleteness(schema *capmodel.ConfigurationSchemaDeclaration, document map[string]any, surface ConfigurationSurface, authorizedMethods []string) error {
	fields, err := validateConfigurationSchema(schema, surface, authorizedMethods)
	if err != nil {
		return err
	}
	if len(document) > maximumConfigurationFields {
		return gerror.New("stored configuration exceeds its bound")
	}
	normalized := make(map[string]any, len(document))
	for key, value := range document {
		if fields[key] == nil {
			return gerror.New("stored configuration has an undeclared field")
		}
		// JSON arrays use []any, while the common condition evaluator consumes
		// typed string selections. Copy the document rather than changing it.
		if values, array := value.([]any); array {
			selections := make([]string, 0, len(values))
			for _, item := range values {
				text, ok := item.(string)
				if !ok {
					return gerror.New("stored selection has an invalid value type")
				}
				selections = append(selections, text)
			}
			normalized[key] = selections
		} else {
			normalized[key] = value
		}
	}
	if schema == nil {
		return nil
	}
	states := initialConfigurationFieldStates(schema, fields)
	if err = applyConfigurationRules(schema, normalized, states); err != nil {
		return err
	}
	if len(configurationStateIssues(schema, normalized, states)) != 0 {
		return gerror.New("stored configuration is incomplete or violates its declaration")
	}
	return nil
}
