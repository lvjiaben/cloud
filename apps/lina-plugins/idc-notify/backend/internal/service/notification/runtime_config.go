package notification

import (
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	publicCustomerOriginEnv = "LINAPRO_IDC_NOTIFY_PUBLIC_CUSTOMER_ORIGIN"
	smsDailyLimitEnv        = "LINAPRO_IDC_NOTIFY_SMS_VERIFICATION_DAILY_LIMIT"
	defaultSMSDailyLimit    = 1000
)

type RuntimeConfig struct {
	PublicCustomerOrigin      string
	SMSVerificationDailyLimit int
}

func LoadRuntimeConfig() (RuntimeConfig, error) {
	config := RuntimeConfig{SMSVerificationDailyLimit: defaultSMSDailyLimit}
	if raw, present := os.LookupEnv(smsDailyLimitEnv); present {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100000 || raw != strings.TrimSpace(raw) {
			return RuntimeConfig{}, gerror.New("notification SMS verification daily limit is invalid")
		}
		config.SMSVerificationDailyLimit = value
	}
	if raw, present := os.LookupEnv(publicCustomerOriginEnv); present && raw != "" {
		if normalized, ok := normalizePublicCustomerOrigin(raw); ok {
			config.PublicCustomerOrigin = normalized
		}
	}
	return config, nil
}

func normalizePublicCustomerOrigin(raw string) (string, bool) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if parsed.Scheme == "http" {
		if !isLoopbackHost(host) {
			return "", false
		}
	} else if parsed.Scheme != "https" || !isPublicHost(host) {
		return "", false
	}
	parsed.Path = ""
	return strings.TrimSuffix(parsed.String(), "/"), true
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

func isPublicHost(host string) bool {
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return false
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() && !address.IsLinkLocalUnicast()
	}
	return net.ParseIP(host) == nil
}
