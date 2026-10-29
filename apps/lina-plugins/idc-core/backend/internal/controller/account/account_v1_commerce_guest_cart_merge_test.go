// These HTTP tests prove credential handling also returns the current owned cart,
// including an explicit empty cart, without exposing anonymous credentials.
package account

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/controller/cartcookie"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

type mergeCartProjectionStub struct {
	commerce.Service
	cart                  *commerce.Cart
	mergeErr, readErr     error
	mergeCalls, readCalls atomic.Int32
}

func (s *mergeCartProjectionStub) GetAccountCart(context.Context) (*commerce.Cart, error) {
	s.readCalls.Add(1)
	return s.cart, s.readErr
}
func (s *mergeCartProjectionStub) MergeGuestCart(context.Context, string) (*commerce.Cart, error) {
	s.mergeCalls.Add(1)
	return s.cart, s.mergeErr
}

func TestGuestMergeReturnsCurrentCartWithoutAdditionalHTTPRead(t *testing.T) {
	for _, name := range []string{"no-credential", "empty-cart", "merged", "expired", "inaccessible", "read-failure", "merge-failure"} {
		t.Run(name, func(t *testing.T) {
			stub := &mergeCartProjectionStub{cart: &commerce.Cart{CartNo: "CRT_current", CurrencyCode: "CNY", Version: 3, Items: []commerce.CartItem{}}}
			if name == "empty-cart" {
				stub.cart = nil
			}
			if name == "expired" {
				stub.mergeErr = bizerr.NewCode(commerce.CodeInvalid)
			}
			if name == "inaccessible" {
				stub.mergeErr = bizerr.NewCode(commerce.CodeNotAccessible)
			}
			if name == "read-failure" {
				stub.readErr = bizerr.NewCode(commerce.CodeUnavailable)
			}
			if name == "merge-failure" {
				stub.mergeErr = bizerr.NewCode(commerce.CodeUnavailable)
			}
			controller := &ControllerV1{commerce: stub}
			server := g.Server(t.Name())
			server.SetAddr("127.0.0.1:0")
			server.SetDumpRouterMap(false)
			server.SetAccessLogEnabled(false)
			server.SetErrorLogEnabled(false)
			server.BindHandler("/merge", func(request *ghttp.Request) {
				result, err := controller.CommerceGuestCartMerge(request.Context(), &v1.CommerceGuestCartMergeReq{})
				if err != nil {
					request.Response.WriteStatus(http.StatusBadRequest)
					return
				}
				request.Response.WriteJson(result)
			})
			if err := server.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := server.Shutdown(); err != nil {
					t.Error(err)
				}
			})
			request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/merge", server.GetListenedPort()), nil)
			if err != nil {
				t.Fatal(err)
			}
			hasCredential := name == "merged" || name == "expired" || name == "inaccessible" || name == "merge-failure"
			if hasCredential {
				request.AddCookie(&http.Cookie{Name: cartcookie.Name, Value: "opaque-browser-credential"})
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if name == "read-failure" || name == "merge-failure" {
				if response.StatusCode != http.StatusBadRequest {
					t.Fatal("owner read/merge failure became an empty successful cart")
				}
				return
			}
			var result map[string]json.RawMessage
			if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			cart, present := result["cart"]
			if !present || (name == "empty-cart") != (string(cart) == "null") {
				t.Fatal("no-credential response omitted the resolved cart state")
			}
			if name != "empty-cart" {
				var current v1.CommerceCart
				if err = json.Unmarshal(cart, &current); err != nil || current.CartNo != stub.cart.CartNo || current.Version != 3 {
					t.Fatal("returned a different customer cart", err)
				}
			}
			wantRead, wantMerge := int32(1), int32(0)
			if hasCredential {
				wantMerge = 1
			}
			if name == "merged" {
				wantRead = 0
			}
			if stub.readCalls.Load() != wantRead || stub.mergeCalls.Load() != wantMerge {
				t.Fatal("duplicated owned projection or merge")
			}
			if hasCredential {
				cookies := response.Cookies()
				if len(cookies) != 1 || cookies[0].Name != cartcookie.Name || cookies[0].MaxAge != -1 {
					t.Fatal("consumed/stale guest credential was not cleared")
				}
			}
		})
	}
}
