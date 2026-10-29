// This file exercises actual GoFrame binding of dynamic server configuration.
package accountnative

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type configurationHTTP struct {
	nativeCommerce
	mu      sync.Mutex
	command customeroperations.ConfigurationCommand
	calls   int
}

func (s *configurationHTTP) Configuration(_ context.Context, c customeroperations.ConfigurationCommand) (*servercommercialcap.ConfigurationContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.command = c
	s.calls++
	value := "9007199254740993"
	return &servercommercialcap.ConfigurationContext{Ready: false, ConfigurationRevision: "opaque-revision", Fields: []servercommercialcap.ConfigurationField{{FieldToken: "field-cpu", Type: "integer", Label: servercommercialcap.ConfigurationLocalizedText{ZhCN: "CPU核心数", EnUS: "CPU cores"}, Constraints: servercommercialcap.ConfigurationConstraints{Minimum: "1", Maximum: "9007199254740993"}, Options: []servercommercialcap.ConfigurationOption{{OptionToken: "opaque-parent", Children: []servercommercialcap.ConfigurationOption{{OptionToken: "opaque-child"}}}}}}, Values: []servercommercialcap.TypedPurchaseValue{{FieldToken: "field-cpu", Type: "integer", IntegerValue: &value}}, Issues: []servercommercialcap.ConfigurationIssue{{FieldToken: "field-cpu", IssueCode: "out_of_range"}}}, nil
}
func (s *configurationHTTP) QuoteTarget(_ context.Context, c customeroperations.ConfigurationCommand) (*servercommercialcap.TargetQuote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.command = c
	s.calls++
	return &servercommercialcap.TargetQuote{QuoteNo: "target-quote", QuoteItemNo: "target-item", StructuredConfigurationNo: "target-config"}, nil
}
func TestNativeConfigurationHTTPPreservesTypedValuesAndRoute(t *testing.T) {
	commercial := &configurationHTTP{}
	controller := &ControllerV1{commercial: commercial}
	server := ghttp.GetServer(guid.S())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.SetAccessLogEnabled(false)
	server.Group("/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(controller.NativeServerConfigurationContext, controller.NativeServerTargetQuote)
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	request := func(method, path, body string) (int, json.RawMessage) {
		t.Helper()
		req, err := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d/api/v1%s", server.GetListenedPort(), path), strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()
		var envelope struct {
			Code int
			Data json.RawMessage
		}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Code, envelope.Data
	}
	number := "SVR-" + strings.Repeat("A", 32)
	path := "/account/native-servers/" + number
	code, data := request(http.MethodGet, path+"/configuration-context?serverNo=SVR-"+strings.Repeat("B", 32), "")
	if code != 0 || !bytes.Contains(data, []byte(`"integerValue":"9007199254740993"`)) || !bytes.Contains(data, []byte(`"maximumInteger":"9007199254740993"`)) || !bytes.Contains(data, []byte(`"optionToken":"opaque-child"`)) || !bytes.Contains(data, []byte(`"billingTermToken":""`)) {
		t.Fatalf("dynamic context corrupt: %d %s", code, data)
	}
	commercial.mu.Lock()
	current := commercial.command
	commercial.mu.Unlock()
	if current.ServerNo != number || current.Values != nil {
		t.Fatal("URL ownership or omitted-values identity lost")
	}
	values := `[{"fieldToken":"field-cpu","type":"integer","integerValue":"9007199254740993"}]`
	if code, _ = request(http.MethodGet, path+"/configuration-context?configurationRevision=revision&values="+url.QueryEscape(values), ""); code != 0 {
		t.Fatal("typed edit rejected")
	}
	commercial.mu.Lock()
	edited := commercial.command
	commercial.mu.Unlock()
	if edited.ConfigurationRevision != "revision" || len(edited.Values) != 1 || *edited.Values[0].IntegerValue != "9007199254740993" {
		t.Fatal("edit changed precision or field token")
	}
	if code, _ = request(http.MethodGet, path+"/configuration-context?configurationRevision=revision&values=%5B%5D", ""); code != 0 {
		t.Fatal("explicit empty edit rejected")
	}
	commercial.mu.Lock()
	empty := commercial.command
	before := commercial.calls
	commercial.mu.Unlock()
	if empty.Values == nil {
		t.Fatal("empty edit became read-current")
	}
	for _, invalid := range []string{`null`, `[null]`, `[{"fieldToken":"a","unknownSupplierKey":"x"}]`, `[] {}`, `not-json`} {
		if code, _ = request(http.MethodGet, path+"/configuration-context?configurationRevision=revision&values="+url.QueryEscape(invalid), ""); code == 0 {
			t.Fatalf("invalid edit accepted %s", invalid)
		}
	}
	if code, _ = request(http.MethodGet, "/account/native-servers/not-a-server/configuration-context?serverNo="+number, ""); code == 0 {
		t.Fatal("query healed invalid URL")
	}
	commercial.mu.Lock()
	after := commercial.calls
	commercial.mu.Unlock()
	if after != before {
		t.Fatal("invalid edit reached core adapter")
	}
	body := `{"serverNo":"SVR-` + strings.Repeat("B", 32) + `","configurationRevision":"revision","billingTermToken":"term","values":` + values + `}`
	if code, data = request(http.MethodPost, path+"/target-quotes", body); code != 0 || !bytes.Contains(data, []byte(`"quoteNo":"target-quote"`)) {
		t.Fatalf("target quote failed: %d %s", code, data)
	}
	commercial.mu.Lock()
	target := commercial.command
	commercial.mu.Unlock()
	if target.ServerNo != number || target.BillingTermToken != "term" || *target.Values[0].IntegerValue != "9007199254740993" {
		t.Fatal("frozen target input changed")
	}
}
