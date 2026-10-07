// Package publicjson validates and canonicalizes bounded JSON that is allowed
// to enter the public/rebuildable version cache. It rejects credential-shaped
// keys and values without storing decoded candidates or secret material.
package publicjson

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Limits bounds the complete parse and credential-decoding workload.
type Limits struct {
	MaximumBytes int
	MaximumDepth int
	MaximumNodes int
}

// Canonicalize validates one object or array and returns deterministic JSON.
// All string scanning shares one document-wide decoded-byte/candidate budget.
func Canonicalize(document []byte, limits Limits) ([]byte, error) {
	if limits.MaximumBytes <= 0 || limits.MaximumDepth <= 0 || limits.MaximumNodes <= 0 ||
		len(document) == 0 || len(document) > limits.MaximumBytes {
		return nil, gerror.New("public cache JSON exceeds configured limits")
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	scanner := newScanner(limits.MaximumBytes, limits.MaximumNodes)
	nodes := 0
	value, err := decodeValue(decoder, scanner, limits, 0, &nodes)
	if err != nil {
		return nil, err
	}
	switch value.(type) {
	case map[string]any, []any:
	default:
		return nil, gerror.New("public cache JSON root must be an object or array")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, gerror.New("public cache JSON contains trailing data")
	}
	canonical, err := json.Marshal(value)
	if err != nil || len(canonical) > limits.MaximumBytes {
		return nil, gerror.New("canonical public cache JSON exceeds configured limits")
	}
	return canonical, nil
}

func decodeValue(
	decoder *json.Decoder,
	scanner *credentialScanner,
	limits Limits,
	depth int,
	nodes *int,
) (any, error) {
	if depth > limits.MaximumDepth || *nodes >= limits.MaximumNodes {
		return nil, gerror.New("public cache JSON exceeds depth or node limits")
	}
	*nodes++
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		if text, ok := token.(string); ok && scanner.contains(text) {
			return nil, gerror.New("public cache JSON contains credential-shaped text")
		}
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return nil, keyErr
			}
			key, ok := keyToken.(string)
			if !ok || ForbiddenKey(key) {
				return nil, gerror.New("public cache JSON contains a forbidden key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, gerror.New("public cache JSON contains a duplicate key")
			}
			child, childErr := decodeValue(decoder, scanner, limits, depth+1, nodes)
			if childErr != nil {
				return nil, childErr
			}
			object[key] = child
		}
		closing, closeErr := decoder.Token()
		if closeErr != nil || closing != json.Delim('}') {
			return nil, gerror.New("public cache JSON object is malformed")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			child, childErr := decodeValue(decoder, scanner, limits, depth+1, nodes)
			if childErr != nil {
				return nil, childErr
			}
			array = append(array, child)
		}
		closing, closeErr := decoder.Token()
		if closeErr != nil || closing != json.Delim(']') {
			return nil, gerror.New("public cache JSON array is malformed")
		}
		return array, nil
	default:
		return nil, gerror.New("public cache JSON delimiter is unsupported")
	}
}
