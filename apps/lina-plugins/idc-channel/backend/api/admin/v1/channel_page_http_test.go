// This file verifies pagination defaults and limits through GoFrame HTTP binding.
package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

type channelPageHTTPProbe struct{}

func (*channelPageHTTPProbe) List(_ context.Context, request *ListChannelsReq) (*ListChannelsRes, error) {
	return &ListChannelsRes{PageMetadata: PageMetadata{Page: request.Page, PageSize: request.PageSize}}, nil
}

func (*channelPageHTTPProbe) Finance(_ context.Context, request *FinanceReq) (*FinanceRes, error) {
	return &FinanceRes{InvoiceTotal: request.InvoicePageSize, StatementTotal: request.StatementPageSize, RepaymentTotal: request.RepaymentPageSize}, nil
}

func TestChannelPageHTTPBinding(t *testing.T) {
	server := g.Server("channel-page-http-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(func(request *ghttp.Request) {
			request.Middleware.Next()
			request.Response.WriteJson(g.Map{"invalid": request.GetError() != nil, "data": request.GetHandlerResponse()})
		})
		group.Bind(&channelPageHTTPProbe{})
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
	base := "http://127.0.0.1:" + strings.TrimPrefix(server.GetListenedAddress(), ":")
	for _, item := range []struct {
		path    string
		invalid bool
	}{
		{"/admin/channel/accounts", false},
		{"/admin/channel/accounts?page=1&pageSize=1", false},
		{"/admin/channel/accounts?page=1000000&pageSize=100", false},
		{"/admin/channel/accounts?page=0", true},
		{"/admin/channel/accounts?page=-1", true},
		{"/admin/channel/accounts?page=1000001", true},
		{"/admin/channel/accounts?pageSize=0", true},
		{"/admin/channel/accounts?pageSize=-1", true},
		{"/admin/channel/accounts?pageSize=101", true},
		{"/admin/channel/finance?channelId=partner", false},
		{"/admin/channel/finance?channelId=partner&invoicePage=0", true},
		{"/admin/channel/finance?channelId=partner&statementPageSize=101", true},
		{"/admin/channel/finance?channelId=partner&repaymentPageSize=-1", true},
	} {
		t.Run(item.path, func(t *testing.T) {
			response, err := client.Get(base + item.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var payload struct {
				Invalid bool `json:"invalid"`
				Data    struct {
					Page           int `json:"page"`
					PageSize       int `json:"pageSize"`
					InvoiceTotal   int `json:"invoiceTotal"`
					StatementTotal int `json:"statementTotal"`
					RepaymentTotal int `json:"repaymentTotal"`
				} `json:"data"`
			}
			if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.Invalid != item.invalid {
				t.Fatalf("invalid=%v, want %v", payload.Invalid, item.invalid)
			}
			if item.path == "/admin/channel/accounts" && (payload.Data.Page != 1 || payload.Data.PageSize != 10) {
				t.Fatalf("unexpected defaults: %+v", payload.Data)
			}
			if item.path == "/admin/channel/finance?channelId=partner" && (payload.Data.InvoiceTotal != 10 || payload.Data.StatementTotal != 10 || payload.Data.RepaymentTotal != 10) {
				t.Fatalf("unexpected independent defaults: %+v", payload.Data)
			}
		})
	}
}
