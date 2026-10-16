// This file proves the GET query format used by the peer connector binds
// typed configuration selections through the real GoFrame request parser.
package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type candidateHTTPProbe struct{}

func (*candidateHTTPProbe) Read(_ context.Context, req *ImportCandidatesReq) (*ImportCandidatesRes, error) {
	result := &ImportCandidatesRes{ImportCandidates: channelcommercecap.ImportCandidates{ConfigurationRevision: req.ConfigurationRevision}}
	var values []channelcommercecap.PurchaseValue
	if err := json.Unmarshal([]byte(req.Values), &values); err != nil {
		return nil, err
	}
	for _, value := range values {
		result.Fields = append(result.Fields, channelcommercecap.ImportCandidateField{FieldToken: value.FieldToken, Options: []channelcommercecap.ImportCandidate{{Value: strings.Join(value.OptionTokens, ",")}}})
	}
	return result, nil
}

func TestImportCandidatesJSONQueryBinding(t *testing.T) {
	server := g.Server("peer-candidate-http-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(func(r *ghttp.Request) {
			r.Middleware.Next()
			r.Response.WriteJson(g.Map{"invalid": r.GetError() != nil, "data": r.GetHandlerResponse()})
		})
		group.Bind(&candidateHTTPProbe{})
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	client := &http.Client{Timeout: 5 * time.Second}
	query := url.Values{"configurationRevision": {"revision"}, "values": {`[{"fieldToken":"field_token","type":"select","optionTokens":["selected"]}]`}}
	response, err := client.Get("http://127.0.0.1:" + strings.TrimPrefix(server.GetListenedAddress(), ":") + "/channel/catalog/products/product/import-candidates?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result struct {
		Invalid bool                `json:"invalid"`
		Data    ImportCandidatesRes `json:"data"`
	}
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Invalid || result.Data.ConfigurationRevision != "revision" || len(result.Data.Fields) != 1 || result.Data.Fields[0].FieldToken != "field_token" || result.Data.Fields[0].Options[0].Value != "selected" {
		t.Fatalf("query lost typed values: %+v", result)
	}
}
