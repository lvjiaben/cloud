package aliyunface

import "testing"

func TestValidHTTPSURLRequiresARealHostname(t *testing.T) {
	t.Parallel()
	if !validHTTPSURL("https://picker.antcloudauth.aliyuncs.com/gateway.do?token=opaque") {
		t.Fatal("expected Aliyun HTTPS continuation URL to be accepted")
	}
	for _, value := range []string{
		"javascript:alert(1)",
		"http://picker.antcloudauth.aliyuncs.com/gateway.do",
		"https://:443/gateway.do",
		"https://user@picker.antcloudauth.aliyuncs.com/gateway.do",
		"https://picker.antcloudauth.aliyuncs.com/gateway.do#fragment",
	} {
		if validHTTPSURL(value) {
			t.Fatalf("unsafe continuation URL %q was accepted", value)
		}
	}
}
