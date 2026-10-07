package resourcecap

import (
	"net"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

func ValidateProviderCorrelation(value *ProviderCorrelation) error {
	if value == nil || value.DownstreamID <= 0 || len(value.DownstreamURL) > 2048 {
		return gerror.New("resource Provider correlation is invalid")
	}
	target, err := url.Parse(value.DownstreamURL)
	if err != nil || target.Hostname() == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" ||
		!strings.HasSuffix(target.Path, "/api/plugins/idc-resource/provider-callback") {
		return gerror.New("resource Provider correlation URL is invalid")
	}
	if target.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(target.Hostname())
	if target.Scheme != "http" || (target.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback())) {
		return gerror.New("resource Provider correlation URL must use HTTPS")
	}
	return nil
}
