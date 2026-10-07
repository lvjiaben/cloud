// This file validates bounded JSON objects without materializing hostile payloads.

package credentialscan

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// maximumJSONBytes bounds one normalized JSON object.
	maximumJSONBytes = 1 << 20
	// maximumJSONDepth bounds nested JSON containers.
	maximumJSONDepth = 64
	// maximumJSONNodes bounds decoded keys, containers, and scalars.
	maximumJSONNodes = 10000
)

// ValidateJSONObject validates one secret-free JSON object using bounded
// streaming traversal and duplicate-key rejection.
func ValidateJSONObject(field string, payload json.RawMessage, required bool) error {
	return validateJSONObject(field, payload, required)
}

// validateJSONObject validates a single bounded JSON object with a streaming
// parser so duplicate keys cannot overwrite an earlier unchecked value.
func validateJSONObject(field string, payload json.RawMessage, required bool) error {
	if len(payload) > maximumJSONBytes {
		return gerror.Newf("resource Provider %s exceeds %d bytes", field, maximumJSONBytes)
	}
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		if required {
			return gerror.Newf("resource Provider %s JSON object is required", field)
		}
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	opening, err := decoder.Token()
	if err != nil {
		return invalidJSONObjectError(field)
	}
	delimiter, ok := opening.(json.Delim)
	if !ok || delimiter != '{' {
		return gerror.Newf("resource Provider %s must be one JSON object", field)
	}
	budget := jsonValidationBudget{}
	if err := budget.consume(0); err != nil {
		return gerror.Wrapf(err, "resource Provider %s is invalid", field)
	}
	if err := validateJSONObjectTokens(decoder, &budget, 0); err != nil {
		return gerror.Wrapf(err, "resource Provider %s is not secret-free", field)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return gerror.Newf("resource Provider %s must contain exactly one JSON object", field)
	}
	return nil
}

// jsonValidationBudget tracks aggregate JSON depth and nodes while tokens are
// consumed without retaining any unvalidated source value.
type jsonValidationBudget struct {
	// nodes is the number of keys, containers, and scalars consumed so far.
	nodes int
}

// consume accounts for one JSON key, container, or scalar at the given depth.
func (b *jsonValidationBudget) consume(depth int) error {
	if depth > maximumJSONDepth {
		return gerror.Newf("JSON exceeds depth %d", maximumJSONDepth)
	}
	b.nodes++
	if b.nodes > maximumJSONNodes {
		return gerror.Newf("JSON exceeds %d nodes", maximumJSONNodes)
	}
	return nil
}

// validateJSONObjectTokens validates an object after its opening delimiter has
// been consumed, including duplicate decoded key rejection.
func validateJSONObjectTokens(decoder *json.Decoder, budget *jsonValidationBudget, depth int) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return gerror.New("JSON object key is invalid")
		}
		key, ok := keyToken.(string)
		if !ok {
			return gerror.New("JSON object key is invalid")
		}
		if err := budget.consume(depth + 1); err != nil {
			return err
		}
		if _, exists := seen[key]; exists {
			return gerror.New("JSON contains a duplicate object key")
		}
		seen[key] = struct{}{}
		if sensitiveJSONKey(key) {
			return gerror.New("sensitive JSON key is not allowed")
		}
		if err := validateJSONValueToken(decoder, budget, depth+1); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		return gerror.New("JSON object is incomplete")
	}
	delimiter, ok := closing.(json.Delim)
	if !ok || delimiter != '}' {
		return gerror.New("JSON object is invalid")
	}
	return nil
}

// validateJSONArrayTokens validates an array after its opening delimiter has
// been consumed.
func validateJSONArrayTokens(decoder *json.Decoder, budget *jsonValidationBudget, depth int) error {
	for decoder.More() {
		if err := validateJSONValueToken(decoder, budget, depth+1); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		return gerror.New("JSON array is incomplete")
	}
	delimiter, ok := closing.(json.Delim)
	if !ok || delimiter != ']' {
		return gerror.New("JSON array is invalid")
	}
	return nil
}

// validateJSONValueToken validates one scalar or recursively consumes one
// nested container without materializing the JSON document.
func validateJSONValueToken(decoder *json.Decoder, budget *jsonValidationBudget, depth int) error {
	token, err := decoder.Token()
	if err != nil {
		return gerror.New("JSON value is invalid")
	}
	if err := budget.consume(depth); err != nil {
		return err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			return validateJSONObjectTokens(decoder, budget, depth)
		case '[':
			return validateJSONArrayTokens(decoder, budget, depth)
		default:
			return gerror.New("JSON value delimiter is invalid")
		}
	case string:
		if containsCredentialMaterial(value) {
			return gerror.New("JSON string contains credential-like material")
		}
	case json.Number, bool, nil:
		return nil
	default:
		return gerror.New("JSON scalar type is invalid")
	}
	return nil
}

// invalidJSONObjectError returns a bounded parse error that never echoes the
// untrusted JSON source.
func invalidJSONObjectError(field string) error {
	return gerror.Newf("resource Provider %s must be valid JSON", field)
}
