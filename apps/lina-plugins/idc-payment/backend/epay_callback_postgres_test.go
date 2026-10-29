// This file runs the generated notification URL through the real HTTP controller,
// EPay verifier and isolated receipt store, without contacting any payment service.
package backend

import (
	"context"
	"crypto/md5" // #nosec G501 -- EPay's documented notification protocol uses MD5.
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	callbackcontroller "lina-plugin-idc-payment/backend/internal/controller/callback"
	"lina-plugin-idc-payment/backend/internal/provider/epay"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
)

func TestEPayGeneratedURLAcceptsSignedGETNotificationPostgreSQL(t *testing.T) {
	db, store, _, _, _, otherFlow := newSubmissionServices(t)
	ctx := context.Background()
	handle, err := secretcap.NewHandle("payment-test-secret-ref")
	if err != nil {
		t.Fatal(err)
	}
	secret := &paymentSecretFake{handle: handle, plaintext: []byte(`{"MerchantID":"1108","secret":"notification-fixture-secret"}`)}
	adapter, err := epay.New("https://epay.example/submit.php")
	if err != nil {
		t.Fatal(err)
	}
	channels, err := channelconfig.New(db, secret, secret, []paymentcap.Adapter{adapter}, func(context.Context, paymentcap.ProviderCode, paymentcap.Credentials) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_payment_channel(channel_no,provider_code,display_name,endpoint_origin,merchant_reference,secret_ref,supported_modes,supported_currencies,capability_snapshot,status_code,verified_at) VALUES('epay-get','epay','EPay test','https://epay.example/submit.php','1108','payment-test-secret-ref','["epay_alipay"]','["CNY"]','{"create":true,"callback":true}','enabled',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	flow, err := paymentflow.New(store, channels, paymentflow.Config{PublicOrigin: "https://merchant.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := flow.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "epay-get", Mode: paymentcap.ModeEPayAlipay})
	if err != nil {
		t.Fatal(err)
	}
	server := g.Server("epay-get-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/x/idc-payment/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(func(r *ghttp.Request) {
			r.Middleware.Next()
			if r.GetError() != nil {
				r.Response.ClearBuffer()
				r.Response.WriteStatus(http.StatusBadRequest, "rejected")
			}
		})
		group.Bind(callbackcontroller.NewV1(flow))
	})
	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if shutdownErr := server.Shutdown(); shutdownErr != nil {
			t.Error(shutdownErr)
		}
	})
	// Use the actual URL emitted by Create, changing only its network origin.
	target, err := url.Parse(created.Redirect.Fields["notify_url"])
	if err != nil || target.Path == "" {
		t.Fatalf("missing generated URL: %v", err)
	}
	endpoint := "http://127.0.0.1:" + strings.TrimPrefix(server.GetListenedAddress(), ":") + target.Path
	values := url.Values{"pid": {"1108"}, "trade_no": {"EPAY_fixture_trade"}, "out_trade_no": {created.AttemptNo}, "type": {"alipay"}, "name": {"云 + server"}, "money": {"123.45"}, "trade_status": {"TRADE_SUCCESS"}, "param": {"invoice-1"}, "sign_type": {"MD5"}}
	signEPayNotification(values)
	client := &http.Client{}
	request := func(method, query, body string, want int) {
		t.Helper()
		req, requestErr := http.NewRequest(method, endpoint+query, strings.NewReader(body))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		response, requestErr := client.Do(req)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		payload, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal(readErr, closeErr)
		}
		if response.StatusCode != want || want == 200 && (string(payload) != "success" || response.Header.Get("Cache-Control") != "no-store") {
			t.Fatalf("%s notification response: %d %q", method, response.StatusCode, payload)
		}
	}
	request(http.MethodGet, "?"+values.Encode(), "", 200)
	request(http.MethodGet, "?"+values.Encode(), "", 200)
	request(http.MethodPost, "", values.Encode(), 200)
	request(http.MethodGet, "?"+values.Encode()+"&pid=1108", "", 400)
	values.Set("sign", strings.Repeat("0", 32))
	request(http.MethodGet, "?"+values.Encode(), "", 400)
	count, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_payment_work_item WHERE work_type='confirm_payment' AND subject_no=?`, created.AttemptNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("notification replay duplicated confirmation: %v %v", count, err)
	}
	if _, _, err = otherFlow.EPayCallback(ctx, "submission-channel", paymentcap.CallbackRequest{}); err == nil {
		t.Fatal("GET admitted a non-EPay provider")
	}
}

// signEPayNotification builds one fixture with the documented decoded-value
// signing rules, independently from the adapter implementation.
func signEPayNotification(values url.Values) {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "sign" && key != "sign_type" && values.Get(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	digest := md5.Sum([]byte(strings.Join(parts, "&") + "notification-fixture-secret")) // #nosec G401 -- protocol fixture.
	values.Set("sign", hex.EncodeToString(digest[:]))
}
