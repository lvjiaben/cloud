// content_scanner_test.go verifies scanner settings stay local and fail closed.
package runtimeconfig

import (
	"testing"
	"time"
)

func TestLoadContentScannerConfigLeavesMissingEndpointUnconfigured(t *testing.T) {
	t.Setenv(ContentScannerEndpointEnvironment, "")
	config, err := LoadContentScannerConfig()
	if err != nil || config.Network != "" || config.Address != "" || config.Timeout != 60*time.Minute {
		t.Fatalf("unexpected unconfigured scanner: %+v err=%v", config, err)
	}
}

func TestLoadContentScannerConfigAcceptsOnlyLocalEndpoints(t *testing.T) {
	tests := []struct {
		name, endpoint, network, address string
	}{
		{name: "IPv4 loopback", endpoint: "tcp://127.0.0.1:3310", network: "tcp", address: "127.0.0.1:3310"},
		{name: "IPv6 loopback", endpoint: "tcp://[::1]:3310", network: "tcp", address: "[::1]:3310"},
		{name: "Unix socket", endpoint: "unix:///run/clamav/clamd.sock", network: "unix", address: "/run/clamav/clamd.sock"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(ContentScannerEndpointEnvironment, test.endpoint)
			t.Setenv(ContentScannerTimeoutEnvironment, "45m")
			t.Setenv(ContentScannerTestedMaxBytesEnvironment, "1073741824")
			config, err := LoadContentScannerConfig()
			if err != nil || config.Network != test.network || config.Address != test.address || config.Timeout != 45*time.Minute || config.TestedMaxFileSizeBytes != 1024*1024*1024 {
				t.Fatalf("config=%+v err=%v", config, err)
			}
		})
	}
}

func TestLoadContentScannerConfigRejectsOutboundOrAmbiguousEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"tcp://clamd.internal:3310",
		"tcp://192.0.2.1:3310",
		"tcp://127.0.0.1",
		"tcp://127.0.0.1:3310/path",
		"http://127.0.0.1:3310",
		"unix://remote/run/clamd.sock",
		"unix:///",
		" tcp://127.0.0.1:3310",
	} {
		t.Run(endpoint, func(t *testing.T) {
			t.Setenv(ContentScannerEndpointEnvironment, endpoint)
			t.Setenv(ContentScannerTestedMaxBytesEnvironment, "1073741824")
			if _, err := LoadContentScannerConfig(); err == nil {
				t.Fatal("unsafe scanner endpoint was accepted")
			}
		})
	}
}

func TestLoadContentScannerConfigRejectsInvalidTimeout(t *testing.T) {
	t.Setenv(ContentScannerEndpointEnvironment, "tcp://127.0.0.1:3310")
	t.Setenv(ContentScannerTestedMaxBytesEnvironment, "1073741824")
	for _, timeout := range []string{"", "0s", "-1s", "60", " 1m"} {
		t.Run(timeout, func(t *testing.T) {
			t.Setenv(ContentScannerTimeoutEnvironment, timeout)
			if _, err := LoadContentScannerConfig(); err == nil {
				t.Fatal("invalid scanner timeout was accepted")
			}
		})
	}
}

func TestLoadContentScannerConfigRequiresBoundedTestedMaximum(t *testing.T) {
	t.Setenv(ContentScannerEndpointEnvironment, "tcp://127.0.0.1:3310")
	for _, limit := range []string{"", "0", "-1", "2147483649", "1GiB", " 1024"} {
		t.Run(limit, func(t *testing.T) {
			t.Setenv(ContentScannerTestedMaxBytesEnvironment, limit)
			if _, err := LoadContentScannerConfig(); err == nil {
				t.Fatal("invalid tested maximum was accepted")
			}
		})
	}
	t.Setenv(ContentScannerEndpointEnvironment, "")
	t.Setenv(ContentScannerTestedMaxBytesEnvironment, "1024")
	if _, err := LoadContentScannerConfig(); err == nil {
		t.Fatal("tested maximum without a local endpoint was accepted")
	}
}
