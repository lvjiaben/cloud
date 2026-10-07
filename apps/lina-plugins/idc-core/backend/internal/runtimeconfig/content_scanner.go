// content_scanner.go validates deployment-only local ClamAV scanner settings.
package runtimeconfig

import (
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	ContentScannerEndpointEnvironment       = "LINAPRO_IDC_CORE_CLAMD_ENDPOINT"
	ContentScannerTimeoutEnvironment        = "LINAPRO_IDC_CORE_CLAMD_TIMEOUT"
	ContentScannerTestedMaxBytesEnvironment = "LINAPRO_IDC_CORE_CLAMD_TESTED_MAX_FILE_BYTES"
	defaultContentScannerTimeout            = 60 * time.Minute
	maximumDangerousContentScannerFileBytes = int64(2 * 1024 * 1024 * 1024)
)

// ContentScannerConfig is the deployment-only local clamd connection.
// An empty Network means that scanning has not been configured.
type ContentScannerConfig struct {
	Network                string
	Address                string
	Timeout                time.Duration
	TestedMaxFileSizeBytes int64
}

// LoadContentScannerConfig accepts only a Unix domain socket or an explicit
// loopback TCP address. clamd TCP has no authentication or encryption, so a
// hostname or non-loopback address must never become an outbound scan path.
func LoadContentScannerConfig() (ContentScannerConfig, error) {
	raw, exists := os.LookupEnv(ContentScannerEndpointEnvironment)
	if !exists || strings.TrimSpace(raw) == "" {
		if _, limitExists := os.LookupEnv(ContentScannerTestedMaxBytesEnvironment); limitExists {
			return ContentScannerConfig{}, gerror.New("IDC content scanner tested maximum requires a local endpoint")
		}
		return ContentScannerConfig{Timeout: defaultContentScannerTimeout}, nil
	}
	if raw != strings.TrimSpace(raw) || len(raw) > 2048 {
		return ContentScannerConfig{}, gerror.New("IDC content scanner endpoint is invalid")
	}
	config, err := parseContentScannerEndpoint(raw)
	if err != nil {
		return ContentScannerConfig{}, err
	}
	config.Timeout = defaultContentScannerTimeout
	if rawTimeout, timeoutExists := os.LookupEnv(ContentScannerTimeoutEnvironment); timeoutExists {
		config.Timeout, err = parseContentScannerTimeout(rawTimeout)
		if err != nil {
			return ContentScannerConfig{}, err
		}
	}
	if rawLimit, limitExists := os.LookupEnv(ContentScannerTestedMaxBytesEnvironment); limitExists {
		config.TestedMaxFileSizeBytes, err = parseContentScannerLimit(rawLimit)
		if err != nil {
			return ContentScannerConfig{}, err
		}
	}
	return config, nil
}

func parseContentScannerEndpoint(raw string) (ContentScannerConfig, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ContentScannerConfig{}, gerror.New("IDC content scanner endpoint is invalid")
	}
	config := ContentScannerConfig{}
	switch parsed.Scheme {
	case "tcp":
		if parsed.Path != "" || parsed.Opaque != "" {
			return ContentScannerConfig{}, gerror.New("IDC content scanner endpoint is invalid")
		}
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		port, portErr := strconv.Atoi(parsed.Port())
		if ip == nil || !ip.IsLoopback() || portErr != nil || port < 1 || port > 65535 {
			return ContentScannerConfig{}, gerror.New("IDC content scanner TCP endpoint must use an explicit loopback address")
		}
		config.Network = "tcp"
		config.Address = net.JoinHostPort(ip.String(), strconv.Itoa(port))
	case "unix":
		if parsed.Host != "" || parsed.Opaque != "" || !strings.HasPrefix(parsed.Path, "/") || strings.Contains(parsed.Path, "\\") {
			return ContentScannerConfig{}, gerror.New("IDC content scanner Unix endpoint is invalid")
		}
		cleaned := path.Clean(parsed.Path)
		if cleaned == "/" {
			return ContentScannerConfig{}, gerror.New("IDC content scanner Unix endpoint is invalid")
		}
		config.Network = "unix"
		config.Address = cleaned
	default:
		return ContentScannerConfig{}, gerror.New("IDC content scanner endpoint must use tcp or unix")
	}
	return config, nil
}

func parseContentScannerTimeout(raw string) (time.Duration, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return 0, gerror.New("IDC content scanner timeout is invalid")
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return 0, gerror.New("IDC content scanner timeout is invalid")
	}
	return timeout, nil
}

func parseContentScannerLimit(raw string) (int64, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return 0, gerror.New("IDC content scanner tested maximum is required")
	}
	limit, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil || limit < 1 || limit > maximumDangerousContentScannerFileBytes {
		return 0, gerror.New("IDC content scanner tested maximum is invalid")
	}
	return limit, nil
}
