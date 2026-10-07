// This file verifies the public sales-channel request contract keeps
// authoritative anti-abuse facts out of caller-controlled JSON.
package v1

import (
	"reflect"
	"strings"
	"testing"
)

func TestSalesChannelAffiliateAttributeDoesNotAcceptRelatedIdentity(t *testing.T) {
	requestType := reflect.TypeOf(SalesChannelAffiliateAttributeReq{})
	for index := 0; index < requestType.NumField(); index++ {
		jsonName := strings.Split(requestType.Field(index).Tag.Get("json"), ",")[0]
		if jsonName == "relatedIdentity" {
			t.Fatal("affiliate attribution request must derive related identity on the server")
		}
	}
}
