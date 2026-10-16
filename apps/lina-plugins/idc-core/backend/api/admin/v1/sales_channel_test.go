// This file verifies the public sales-channel request contract keeps
// authoritative anti-abuse facts out of caller-controlled JSON.
package v1

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestSalesChannelActiveCalculationsValidateCurrencyLength(t *testing.T) {
	for _, currency := range []string{"CNY", "USD", "", "CN", "CNYY"} {
		requests := map[string]any{
			"offer": &SalesChannelOfferEvaluateReq{CustomerNo: "customer", ProductNo: "product", CurrencyCode: currency, NowUnixMilli: 1},
			"commission": &SalesChannelCommissionCalculateReq{CurrencyCode: currency, NowUnixMilli: 1,
				OperationTypeCode: "new_purchase", BeneficiaryTypeCode: "customer", RuleNo: "rule",
				CalculationTypeCode: "fixed", RuleVersion: 1, EffectiveAtUnixMilli: 1},
		}
		for name, request := range requests {
			t.Run(name+"/"+currency, func(t *testing.T) {
				err := g.Validator().Data(request).Run(context.Background())
				if valid := len(currency) == 3; valid && err != nil {
					t.Fatalf("valid currency was rejected: %v", err)
				} else if !valid && err == nil {
					t.Fatal("invalid currency length was accepted")
				}
			})
		}
	}
}

func TestSalesChannelAffiliateAttributeDoesNotAcceptRelatedIdentity(t *testing.T) {
	requestType := reflect.TypeOf(SalesChannelAffiliateAttributeReq{})
	for index := 0; index < requestType.NumField(); index++ {
		jsonName := strings.Split(requestType.Field(index).Tag.Get("json"), ",")[0]
		if jsonName == "relatedIdentity" {
			t.Fatal("affiliate attribution request must derive related identity on the server")
		}
	}
}
