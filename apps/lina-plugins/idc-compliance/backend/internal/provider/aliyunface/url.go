// This file validates provider and merchant return URLs without introducing a
// configurable outbound endpoint or an SSRF-capable redirect surface.

package aliyunface

import "net/url"

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.Hostname() != "" &&
		parsed.User == nil && parsed.Fragment == ""
}
