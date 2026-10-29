// This file compares settings semantically so JSONB formatting never creates a
// new connection revision when an administrator saves unchanged information.
package providerconfiguration

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// sameNonSecretSettings compares decoded values without losing integer precision.
func sameNonSecretSettings(left, right json.RawMessage) bool {
	if !json.Valid(left) || !json.Valid(right) {
		return false
	}
	var leftValue, rightValue any
	leftDecoder := json.NewDecoder(bytes.NewReader(left))
	leftDecoder.UseNumber()
	rightDecoder := json.NewDecoder(bytes.NewReader(right))
	rightDecoder.UseNumber()
	if leftDecoder.Decode(&leftValue) != nil || rightDecoder.Decode(&rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}
