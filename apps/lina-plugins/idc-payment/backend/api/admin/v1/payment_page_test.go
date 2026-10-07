// This file verifies numeric pagination bounds and unchanged read permissions.
package v1

import (
	"context"
	"reflect"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestPaymentPageValidationAndPermissions(t *testing.T) {
	cases := []struct {
		name, permission string
		value            any
	}{
		{"channels", "idc:admin:payment:channel:view", &PaymentChannelListReq{}},
		{"health", "idc:admin:payment:reconciliation:view", &ReconciliationHealthReq{}},
		{"batches", "idc:admin:payment:reconciliation:view", &ReconciliationBatchListReq{}},
		{"differences", "idc:admin:payment:reconciliation:view", &ReconciliationDifferenceListReq{BatchNo: "batch-local"}},
		{"funds", "idc:admin:payment:exception-fund:view", &ExceptionFundListReq{}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			value := reflect.ValueOf(item.value).Elem()
			meta, _ := value.Type().FieldByName("Meta")
			if meta.Tag.Get("permission") != item.permission {
				t.Fatalf("read permission changed: %s", meta.Tag.Get("permission"))
			}
			for _, size := range []int{10, 20, 50, 100} {
				value.FieldByName("Page").SetInt(1)
				value.FieldByName("PageSize").SetInt(int64(size))
				if err := g.Validator().Data(item.value).Run(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			for _, invalid := range [][2]int{{0, 20}, {1, 0}, {1, 101}, {1_000_001, 20}} {
				value.FieldByName("Page").SetInt(int64(invalid[0]))
				value.FieldByName("PageSize").SetInt(int64(invalid[1]))
				if err := g.Validator().Data(item.value).Run(context.Background()); err == nil {
					t.Fatal("invalid page accepted", invalid)
				}
			}
		})
	}
}
