// This file applies only explicit bindings from the immutable sale configuration.
// Unbound customer information stays out of provider input, and no live product
// edit can alter a paid order's parameter mapping or execution request digest.
package productfulfillment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func (row *claimedFulfillment) boundProductConfiguration() (json.RawMessage, string, error) {
	presentations, err := configurationcontract.DecodePresentations(row.CustomerPresentation)
	if err != nil {
		return nil, "", err
	}
	bound := false
	for _, presentation := range presentations {
		if presentation.ProductFieldKey != "" {
			bound = true
			break
		}
	}
	if !bound {
		return row.ProductConfiguration, row.ProductConfigurationDigest, nil
	}
	declaration, err := configurationcontract.DecodeDeclaration(row.CustomerDeclaration)
	if err != nil || declaration == nil {
		return nil, "", gerror.New("bound purchase declaration is unavailable")
	}
	productDeclaration, err := configurationcontract.DecodeDeclaration(row.ProductDeclaration)
	if err != nil {
		return nil, "", gerror.New("bound product declaration is unavailable")
	}
	targets := map[string]string{}
	for _, field := range productFields(productDeclaration) {
		if field != nil {
			targets[field.Key] = field.Type
		}
	}
	var (
		product  = map[string]json.RawMessage{}
		customer = map[string]json.RawMessage{}
		declared = map[string]string{}
		used     = map[string]bool{}
		custom   = map[string]json.RawMessage{}
	)
	if err = json.Unmarshal(row.ProductConfiguration, &product); err != nil || product == nil {
		return nil, "", gerror.New("bound product input is unavailable")
	}
	if err = json.Unmarshal(row.CustomerConfiguration, &customer); err != nil || customer == nil {
		return nil, "", gerror.New("bound customer input is unavailable")
	}
	for _, field := range declaration.Fields {
		if field != nil && field.Required && field.Type != "file" && field.Type != "secret" {
			declared[field.Key] = field.Type
		}
	}
	for key, presentation := range presentations {
		target := presentation.ProductFieldKey
		if target == "" {
			continue
		}
		value, exists := customer[key]
		if !strings.HasPrefix(key, "local_") || declared[key] == "" || !exists || string(value) == "null" || used[target] {
			return nil, "", gerror.New("frozen purchase parameter binding is invalid")
		}
		// Declared fields use their frozen technical schema. Explicit native
		// names use their saved mapping; neither path reads a live product.
		if targets[target] == "" {
			if productfulfillmentcap.ValidateCustomParameterName(target) != nil {
				return nil, "", gerror.New("frozen custom parameter name is invalid")
			}
			value, err = boundProductChoiceValue(presentation, value)
			if err != nil {
				return nil, "", err
			}
			custom[target] = value
			used[target] = true
			continue
		}
		if targets[target] != declared[key] && !(targets[target] == "single_line" && declared[key] == "single_select") {
			return nil, "", gerror.New("frozen product parameter is missing")
		}
		used[target] = true
		value, err = boundProductChoiceValue(presentation, value)
		if err != nil {
			return nil, "", err
		}
		product[target] = value
	}
	return encodeBoundProductConfiguration(product, custom)
}

// encodeBoundProductConfiguration validates the explicit native envelope and
// hashes the complete execution input without mutating the frozen source.
func encodeBoundProductConfiguration(product, custom map[string]json.RawMessage) (json.RawMessage, string, error) {
	if len(custom) > 0 {
		encoded, encodeErr := json.Marshal(custom)
		if encodeErr != nil {
			return nil, "", encodeErr
		}
		product[productfulfillmentcap.CustomParametersKey] = encoded
	}
	result, err := json.Marshal(product)
	if err != nil {
		return nil, "", err
	}
	if _, err = productfulfillmentcap.CustomParameters(result); err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(result)
	return result, hex.EncodeToString(digest[:]), nil
}

func boundProductChoiceValue(presentation configurationcontract.Presentation, value json.RawMessage) (json.RawMessage, error) {
	if len(presentation.ProductChoiceBindings) == 0 {
		return value, nil
	}
	var choice string
	if json.Unmarshal(value, &choice) != nil {
		return nil, gerror.New("frozen choice mapping value is invalid")
	}
	return json.Marshal(configurationcontract.ProductChoiceValue(presentation.ProductChoiceBindings, choice))
}

// productFields treats an omitted semantic schema as empty for custom-only integrations.
func productFields(declaration *capmodel.ConfigurationSchemaDeclaration) []*capmodel.ConfigurationFieldDeclaration {
	if declaration == nil {
		return nil
	}
	return declaration.Fields
}
