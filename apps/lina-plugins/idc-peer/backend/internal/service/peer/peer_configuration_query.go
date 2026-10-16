// This file serializes bounded configuration reads as URL query JSON.
package peer

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func configurationQuery(input channelcommercecap.ConfigurationInput) (map[string][]string, error) {
	if input.Validate() != nil || len(input.Values) > 128 {
		return nil, errInvalid
	}
	values, err := json.Marshal(input.Values)
	if input.Values == nil {
		values = []byte("[]")
	}
	if err != nil || len(values) > 32768 {
		return nil, errInvalid
	}
	return map[string][]string{"configurationRevision": {input.ConfigurationRevision}, "currencyCode": {input.CurrencyCode}, "values": {string(values)}}, nil
}
