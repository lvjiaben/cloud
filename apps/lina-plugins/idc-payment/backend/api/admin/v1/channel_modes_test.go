// This file verifies the HTTP validator accepts mode arrays as arrays; the
// service enforces the bounded count rather than a serialized string length.
package v1

import (
	"context"
	"github.com/gogf/gf/v2/util/gvalid"
	"testing"
)

func TestChannelModeRequestAcceptsMultipleNamedModes(t *testing.T) {
	req := PaymentChannelModesSaveReq{ChannelNo: "EPAY_MAIN", EnabledModes: []string{"epay_alipay", "epay_wechat"}, ExpectedVersion: 1}
	if err := gvalid.New().Data(req).Run(context.Background()); err != nil {
		t.Fatalf("valid mode array rejected: %v", err)
	}
}
