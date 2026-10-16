// This file permits only opaque same-system schema field keys at their exact
// authenticated import location. Other key fields remain forbidden and all
// ordinary credential echo screening remains active.

package composition

import (
	"encoding/json"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"net/http"
	"regexp"
	"strings"
)

var peerImportPath = regexp.MustCompile(`^/x/idc-channel/api/v1/channel/catalog/products/[a-z0-9][a-z0-9-]{0,159}/import-snapshot$`)
var peerSchemaKey = regexp.MustCompile(`^k_[a-f0-9]{48}$`)

func peerImportSchemaRequest(request resourcecap.ConfidentialHTTPRequest) bool {
	return request.Method == http.MethodGet && request.UseAuthentication && peerImportPath.MatchString(request.Path)
}

func validatePeerImportSchemaKeys(request resourcecap.ConfidentialHTTPRequest, content []byte) error {
	if !peerImportSchemaRequest(request) {
		return gerror.New("peer import path is invalid")
	}
	var value any
	if json.Unmarshal(content, &value) != nil {
		return gerror.New("peer import response is invalid")
	}
	var visit func(any, string) bool
	visit = func(value any, path string) bool {
		switch item := value.(type) {
		case map[string]any:
			for name, child := range item {
				if strings.EqualFold(name, "key") {
					key, ok := child.(string)
					if name != "key" || path != "data.schema.fields[]" || !ok || !peerSchemaKey.MatchString(key) {
						return false
					}
				}
				childPath := name
				if path != "" {
					childPath = path + "." + name
				}
				if !visit(child, childPath) {
					return false
				}
			}
		case []any:
			for _, child := range item {
				if !visit(child, path+"[]") {
					return false
				}
			}
		}
		return true
	}
	if !visit(value, "") {
		return gerror.New("peer schema key is not opaque or correctly scoped")
	}
	return nil
}
