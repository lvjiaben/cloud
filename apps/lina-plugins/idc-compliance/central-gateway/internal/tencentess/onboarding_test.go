// This file verifies official Tencent onboarding request mapping and subject isolation.

package tencentess

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"
)

type onboardingOfficialAPI struct {
	*fakeOfficialAPI
	request *essbasic.CreateConsoleLoginUrlRequest
}

func (f *onboardingOfficialAPI) CreateConsoleLoginUrlWithContext(
	_ context.Context,
	request *essbasic.CreateConsoleLoginUrlRequest,
) (*essbasic.CreateConsoleLoginUrlResponse, error) {
	f.request = request
	response := essbasic.NewCreateConsoleLoginUrlResponse()
	response.Response = &essbasic.CreateConsoleLoginUrlResponseParams{
		ConsoleUrl:              common.StringPtr("https://qian.tencent.com/console/one-use"),
		IsActivated:             common.BoolPtr(true),
		ProxyOperatorIsVerified: common.BoolPtr(true),
		RequestId:               common.StringPtr("request-onboarding"),
	}
	return response, nil
}

func TestCreateOrganizationConsoleURLKeepsSensitiveFieldsOnTencent(t *testing.T) {
	api := &onboardingOfficialAPI{fakeOfficialAPI: inertAPI()}
	client := testClient(t, api, nil, CapabilityTemplateRead)
	result, err := client.CreateOrganizationConsoleURL(
		context.Background(), "linst_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"示例 IDC 有限公司",
	)
	if err != nil {
		t.Fatalf("CreateOrganizationConsoleURL returned error: %v", err)
	}
	if result.ConsoleURL == "" || !result.Activated || !result.OperatorVerified || api.request == nil ||
		api.request.UniformSocialCreditCode != nil || api.request.ProxyOperatorName != nil ||
		api.request.ProxyOperatorMobile != nil || api.request.ProxyOperatorIdCardNumber != nil ||
		stringValue(api.request.ProxyOrganizationName) != "示例 IDC 有限公司" ||
		stringValue(api.request.Endpoint) != "PC" || !validAgent(api.request.Agent) {
		t.Fatalf("unexpected onboarding request/result: request=%+v result=%+v", api.request, result)
	}
	authorization := result.ConsumeAuthorization()
	defer clearBytes(authorization)
	var subject canonicalSubject
	if err = json.Unmarshal(authorization, &subject); err != nil ||
		subject.ProxyOrganizationOpenID == "" || subject.ProxyOperatorOpenID == "" {
		t.Fatalf("invalid private authorization result: subject=%+v err=%v", subject, err)
	}
	if second := result.ConsumeAuthorization(); second != nil {
		t.Fatalf("authorization result was consumable twice: %q", second)
	}
}

func TestCreateOrganizationConsoleURLUsesTencentCharacterLimit(t *testing.T) {
	api := &onboardingOfficialAPI{fakeOfficialAPI: inertAPI()}
	client := testClient(t, api, nil, CapabilityTemplateRead)
	installationID := "linst_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if _, err := client.CreateOrganizationConsoleURL(
		context.Background(), installationID, strings.Repeat("企", maximumLegalNameCharacters),
	); err != nil {
		t.Fatalf("200-character legal name was rejected: %v", err)
	}
	if _, err := client.CreateOrganizationConsoleURL(
		context.Background(), installationID, strings.Repeat("企", maximumLegalNameCharacters+1),
	); err == nil {
		t.Fatal("201-character legal name was accepted")
	}
}
