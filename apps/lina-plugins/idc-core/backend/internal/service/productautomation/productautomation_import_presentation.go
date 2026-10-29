// Portable imports preserve only the source customer visibility/default policy.
// Provider bindings and local rendering metadata never cross this boundary.
package productautomation

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func portableCustomerPresentationJSON(snapshot *channelcommercecap.ImportSnapshot) ([]byte, error) {
	presentations := map[string]configurationcontract.Presentation{}
	validationPresentations := map[string]configurationcontract.Presentation{}
	if snapshot != nil {
		for _, mapping := range snapshot.Mappings {
			if mapping.CustomerVisible == nil && len(mapping.DefaultValue) == 0 {
				continue
			}
			presentations[mapping.FieldKey] = configurationcontract.Presentation{CustomerVisible: mapping.CustomerVisible, DefaultValue: append(json.RawMessage(nil), mapping.DefaultValue...)}
			checked := presentations[mapping.FieldKey]
			if len(checked.DefaultValue) > 0 {
				hidden := false
				checked.CustomerVisible = &hidden
			}
			validationPresentations[mapping.FieldKey] = checked
		}
		if err := configurationcontract.ValidatePresentations(snapshot.Schema, validationPresentations); err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(presentations)
	if err != nil {
		return nil, err
	}
	// Persist only documents admitted by the same bounded reader used later.
	if _, err = configurationcontract.DecodePresentations(string(raw)); err != nil {
		return nil, err
	}
	return raw, nil
}

func verifyPortableCustomerPresentation(snapshot *channelcommercecap.ImportSnapshot, actual string) error {
	expectedJSON, err := portableCustomerPresentationJSON(snapshot)
	if err != nil {
		return err
	}
	expected, err := configurationcontract.DecodePresentations(string(expectedJSON))
	if err != nil {
		return err
	}
	got, err := configurationcontract.DecodePresentations(actual)
	if err != nil {
		return err
	}
	// Imported configuration is immutable; accepting extra metadata could alter
	// numeric price inclusion, provisioning mappings or hidden defaults.
	actualPolicy, err := canonicalPortablePresentation(got)
	if err != nil {
		return err
	}
	expectedPolicy, err := canonicalPortablePresentation(expected)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actualPolicy, expectedPolicy) {
		return gerror.New("portable import customer policy mismatch")
	}
	return nil
}

func canonicalPortablePresentation(value map[string]configurationcontract.Presentation) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized any
	if err = decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}
