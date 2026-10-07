package account

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

type creditHTTPBoundaryCommandStub struct {
	billing.AccountCreditCommandService
	acceptedCounts []int
}

func (s *creditHTTPBoundaryCommandStub) SubmitCreditApplication(
	_ context.Context,
	intent billing.AccountCreditApplicationIntent,
) (*billing.CreditApplication, error) {
	s.acceptedCounts = append(s.acceptedCounts, len(intent.UploadNos))
	return nil, bizerr.NewCode(billing.CodeCreditConflict)
}

type creditHTTPBoundaryController struct{ target *ControllerV1 }

func (c *creditHTTPBoundaryController) Submit(
	ctx context.Context,
	req *v1.CreditApplicationSubmitReq,
) (*v1.CreditApplicationSubmitRes, error) {
	return c.target.CreditApplicationSubmit(ctx, req)
}

type creditHTTPBoundaryResponse struct {
	ErrorCode string `json:"errorCode"`
}

func TestCreditApplicationUploadCardinalityThroughStartedHTTPRoute(t *testing.T) {
	command := &creditHTTPBoundaryCommandStub{}
	server := g.Server("idc-credit-http-boundary-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/x/idc-core/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(func(request *ghttp.Request) {
			request.Middleware.Next()
			if structured, ok := bizerr.As(request.GetError()); ok {
				request.Response.ClearBuffer()
				request.Response.WriteJson(g.Map{"errorCode": structured.RuntimeCode()})
			}
		})
		group.Bind(&creditHTTPBoundaryController{target: &ControllerV1{creditCommands: command}})
	})
	if err := server.Start(); err != nil {
		t.Fatalf("start credit boundary server: %v", err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Fatalf("shutdown credit boundary server: %v", err)
		}
	})
	time.Sleep(100 * time.Millisecond)

	wantCodes := map[int]string{
		0:  billing.CodeCreditInvalid.RuntimeCode(),
		1:  billing.CodeCreditConflict.RuntimeCode(),
		10: billing.CodeCreditConflict.RuntimeCode(),
		11: billing.CodeCreditInvalid.RuntimeCode(),
	}
	for _, uploadCount := range []int{0, 1, 10, 11} {
		uploadCount := uploadCount
		t.Run(fmt.Sprintf("uploads_%d", uploadCount), func(t *testing.T) {
			uploadNos := make([]string, uploadCount)
			for index := range uploadNos {
				uploadNos[index] = fmt.Sprintf("CEU_%032d", index+1)
			}
			requestBody, err := json.Marshal(g.Map{
				"currencyCode": "CNY", "requestedLimitMinor": "100000",
				"requestedValidFrom":  int64(1_800_000_000_000),
				"requestedValidUntil": int64(1_900_000_000_000),
				"uploadNos":           uploadNos, "idempotencyKey": fmt.Sprintf("credit-http-%d", uploadCount),
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.Post(
				"http://127.0.0.1:"+strings.TrimPrefix(server.GetListenedAddress(), ":")+"/x/idc-core/api/v1/account/billing/credit/applications",
				"application/json",
				bytes.NewReader(requestBody),
			)
			if err != nil {
				t.Fatalf("post credit boundary request: %v", err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read credit boundary response: %v", err)
			}
			if uploadCount == 0 {
				if len(body) != 0 {
					t.Fatalf("required collection binding should reject before controller, got body=%s", body)
				}
				return
			}
			var result creditHTTPBoundaryResponse
			if err = json.Unmarshal(body, &result); err != nil {
				t.Fatalf("decode credit boundary response %s: %v", body, err)
			}
			if result.ErrorCode != wantCodes[uploadCount] {
				t.Fatalf("uploads=%d expected %s, got %s body=%s", uploadCount, wantCodes[uploadCount], result.ErrorCode, body)
			}
		})
	}
	if fmt.Sprint(command.acceptedCounts) != "[1 10]" {
		t.Fatalf("only valid collection bounds may reach business service, got %v", command.acceptedCounts)
	}
}
