// This file prevents generic validator errors from interpolating private server
// passwords before the registry can return its stable, secret-free bizerr.
package api

import (
	"github.com/gogf/gf/v2/util/gvalid"
	accountv1 "lina-plugin-idc-core/backend/api/account/v1"
	"testing"
)

func TestPrivatePasswordSkipsValueInterpolatingGenericValidation(t *testing.T) {
	request := &accountv1.PurchaseServerLoginCreateReq{PublicSlug: "cloud-server", ConfigurationRevision: "revision", Password: "NeverEchoThisOverlongPassword123!"}
	if err := gvalid.New().Data(request).Run(t.Context()); err != nil {
		t.Fatal("password must reach the service's secret-free validation boundary")
	}
}
