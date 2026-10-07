package v1

import (
	"reflect"
	"strings"
	"testing"
)

func TestCustomerPreferenceContractDoesNotOfferWeCom(t *testing.T) {
	field, ok := reflect.TypeOf(PreferenceUpdateReq{}).FieldByName("ChannelCode")
	if !ok {
		t.Fatal("preference channel field is missing")
	}
	validation := field.Tag.Get("v")
	description := field.Tag.Get("dc")
	if strings.Contains(validation, "wechat_work") || !strings.Contains(description, "internal staff only") {
		t.Fatalf("customer channel contract still offers WeCom: v=%q dc=%q", validation, description)
	}
	deliveryField, ok := reflect.TypeOf(DeliveryItem{}).FieldByName("ChannelCode")
	if !ok || strings.Contains(deliveryField.Tag.Get("dc"), "wechat_work") {
		t.Fatalf("customer delivery contract still offers WeCom: %+v", deliveryField)
	}
}
