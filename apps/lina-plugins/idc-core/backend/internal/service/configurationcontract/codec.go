// This file strictly decodes frozen structured-configuration declarations.

package configurationcontract

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

// DecodeDeclaration rejects unknown fields, trailing JSON, and malformed
// snapshots before a frozen plugin contract reaches a public boundary.
func DecodeDeclaration(value string) (*capmodel.ConfigurationSchemaDeclaration, error) {
	if value == "" || value == "{}" {
		return nil, nil
	}
	var schema capmodel.ConfigurationSchemaDeclaration
	decoder := json.NewDecoder(bytes.NewReader([]byte(value)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&schema); err != nil {
		return nil, gerror.Wrap(err, "decode configuration declaration")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, gerror.New("configuration declaration contains trailing data")
	}
	if schema.Version == "" {
		return nil, nil
	}
	return &schema, nil
}
