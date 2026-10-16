// This file verifies that the peer import key exception cannot reveal arbitrary
// credential fields or accept plaintext outside the exact schema location.

package composition

import (
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"strings"
	"testing"
)

func TestPeerImportAllowsOnlyOpaqueSchemaFieldKeys(t *testing.T) {
	request := resourcecap.ConfidentialHTTPRequest{Method: "GET", UseAuthentication: true, Path: "/x/idc-channel/api/v1/channel/catalog/products/demo/import-snapshot"}
	key := "k_" + strings.Repeat("a", 48)
	valid := `{"code":0,"data":{"schema":{"fields":[{"key":"` + key + `","type":"single_line"}]}}}`
	if err := validatePeerImportSchemaKeys(request, []byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"code":0,"data":{"schema":{"fields":[{"key":"app-secret-plaintext"}]}}}`,
		`{"code":0,"data":{"key":"` + key + `"}}`,
		`{"code":0,"data":{"schema":{"fields":[{"Key":"` + key + `"}]}}}`,
	} {
		if validatePeerImportSchemaKeys(request, []byte(raw)) == nil {
			t.Fatal("unsafe key projection accepted")
		}
	}
	request.Path = "/other/import-snapshot"
	if validatePeerImportSchemaKeys(request, []byte(valid)) == nil {
		t.Fatal("non-peer endpoint accepted")
	}
}
