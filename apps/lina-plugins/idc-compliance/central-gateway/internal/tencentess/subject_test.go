// This file tests the server-only Tencent subject projection and active-subject
// adapter boundary.

package tencentess

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSubjectCanonicalizerAcceptsOnlyClosedAuthorizationProjection(t *testing.T) {
	canonicalizer := SubjectCanonicalizer{}
	result, err := canonicalizer.Canonicalize(
		context.Background(),
		[]byte(`{"proxyOperatorOpenId":"operator-id","proxyOrganizationOpenId":"organization-id"}`),
	)
	if err != nil {
		t.Fatalf("Canonicalize returned error: %v", err)
	}
	defer clearBytes(result)
	var subject canonicalSubject
	if err = json.Unmarshal(result, &subject); err != nil ||
		subject.ProxyOrganizationOpenID != "organization-id" ||
		subject.ProxyOperatorOpenID != "operator-id" {
		t.Fatalf("unexpected canonical subject: %s err=%v", result, err)
	}
	for _, invalid := range [][]byte{
		[]byte(`{"proxyOrganizationOpenId":"organization-id"}`),
		[]byte(`{"proxyOrganizationOpenId":"organization-id","proxyOperatorOpenId":"operator-id","manual":true}`),
		[]byte(`{"proxyOrganizationOpenId":" organization-id","proxyOperatorOpenId":"operator-id"}`),
	} {
		if _, err = canonicalizer.Canonicalize(context.Background(), invalid); err == nil {
			t.Fatalf("invalid authorization projection was accepted: %s", invalid)
		}
	}
}
