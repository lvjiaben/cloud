// Package app owns the installation-local compliance gateway process boundary.
package app

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

// Environment names and process bounds are fixed by the local delivery contract.
const (
	listenAddressEnv               = "LINAPRO_COMPLIANCE_GATEWAY_LISTEN_ADDR"
	readHeaderTimeoutEnv           = "LINAPRO_COMPLIANCE_GATEWAY_READ_HEADER_TIMEOUT"
	readTimeoutEnv                 = "LINAPRO_COMPLIANCE_GATEWAY_READ_TIMEOUT"
	writeTimeoutEnv                = "LINAPRO_COMPLIANCE_GATEWAY_WRITE_TIMEOUT"
	idleTimeoutEnv                 = "LINAPRO_COMPLIANCE_GATEWAY_IDLE_TIMEOUT"
	shutdownTimeoutEnv             = "LINAPRO_COMPLIANCE_GATEWAY_SHUTDOWN_TIMEOUT"
	databaseDSNFileEnv             = "LINAPRO_COMPLIANCE_GATEWAY_DATABASE_DSN_FILE"
	installationIDFileEnv          = "LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE"
	processAuthKeyringFileEnv      = "LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE"
	supplierIdentityKeyringFileEnv = "LINAPRO_COMPLIANCE_GATEWAY_SUPPLIER_IDENTITY_KEYRING_FILE"
	callbackEvidenceKeyringFileEnv = "LINAPRO_COMPLIANCE_GATEWAY_CALLBACK_EVIDENCE_KEYRING_FILE"
	tencentESSBasicConfigFileEnv   = "LINAPRO_COMPLIANCE_GATEWAY_TENCENT_ESSBASIC_CONFIG_FILE"
	processAuthWindowEnv           = "LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW"
	defaultListenAddress           = "127.0.0.1:9088"
	defaultHeaderTimeout           = 5 * time.Second
	defaultReadTimeout             = 10 * time.Second
	defaultWriteTimeout            = 10 * time.Second
	defaultIdleTimeout             = 30 * time.Second
	defaultShutdownTimeout         = 10 * time.Second
	minimumTimeout                 = time.Second
	maximumTimeout                 = 2 * time.Minute
	maximumHeaderBytes             = 64 << 10
	maximumConfigurationPathBytes  = 4096
)

// RuntimeConfig references only files owned by the current installation.
type RuntimeConfig struct {
	DatabaseDSNFile             string
	InstallationIDFile          string
	ProcessAuthKeyringFile      string
	SupplierIdentityKeyringFile string
	CallbackEvidenceKeyringFile string
	TencentESSBasicConfigFile   string
	ProcessAuthWindow           time.Duration
}

// Config contains process-level settings and the mandatory local runtime.
type Config struct {
	ListenAddress     string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
	Runtime           RuntimeConfig
}

// LoadConfig reads bounded operational settings from the supplied environment lookup.
func LoadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, fmt.Errorf("environment lookup is required")
	}
	config := Config{
		ListenAddress:     valueOrDefault(getenv(listenAddressEnv), defaultListenAddress),
		ReadHeaderTimeout: defaultHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		ShutdownTimeout:   defaultShutdownTimeout,
		MaxHeaderBytes:    maximumHeaderBytes,
	}
	var err error
	for _, item := range []struct {
		name   string
		target *time.Duration
	}{
		{name: readHeaderTimeoutEnv, target: &config.ReadHeaderTimeout},
		{name: readTimeoutEnv, target: &config.ReadTimeout},
		{name: writeTimeoutEnv, target: &config.WriteTimeout},
		{name: idleTimeoutEnv, target: &config.IdleTimeout},
		{name: shutdownTimeoutEnv, target: &config.ShutdownTimeout},
	} {
		*item.target, err = durationOrDefault(getenv(item.name), *item.target)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", item.name, err)
		}
	}
	if err := validateListenAddress(config.ListenAddress); err != nil {
		return Config{}, err
	}
	config.Runtime = RuntimeConfig{
		DatabaseDSNFile:             strings.TrimSpace(getenv(databaseDSNFileEnv)),
		InstallationIDFile:          strings.TrimSpace(getenv(installationIDFileEnv)),
		ProcessAuthKeyringFile:      strings.TrimSpace(getenv(processAuthKeyringFileEnv)),
		SupplierIdentityKeyringFile: strings.TrimSpace(getenv(supplierIdentityKeyringFileEnv)),
		CallbackEvidenceKeyringFile: strings.TrimSpace(getenv(callbackEvidenceKeyringFileEnv)),
		TencentESSBasicConfigFile:   strings.TrimSpace(getenv(tencentESSBasicConfigFileEnv)),
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{name: databaseDSNFileEnv, value: config.Runtime.DatabaseDSNFile},
		{name: installationIDFileEnv, value: config.Runtime.InstallationIDFile},
		{name: processAuthKeyringFileEnv, value: config.Runtime.ProcessAuthKeyringFile},
		{name: supplierIdentityKeyringFileEnv, value: config.Runtime.SupplierIdentityKeyringFile},
		{name: callbackEvidenceKeyringFileEnv, value: config.Runtime.CallbackEvidenceKeyringFile},
	} {
		if err = validateConfigurationFilePath(item.name, item.value); err != nil {
			return Config{}, err
		}
	}
	if config.Runtime.TencentESSBasicConfigFile != "" {
		if err = validateConfigurationFilePath(
			tencentESSBasicConfigFileEnv, config.Runtime.TencentESSBasicConfigFile,
		); err != nil {
			return Config{}, err
		}
	}
	paths := []string{
		filepath.Clean(config.Runtime.DatabaseDSNFile),
		filepath.Clean(config.Runtime.InstallationIDFile),
		filepath.Clean(config.Runtime.ProcessAuthKeyringFile),
		filepath.Clean(config.Runtime.SupplierIdentityKeyringFile),
		filepath.Clean(config.Runtime.CallbackEvidenceKeyringFile),
	}
	if config.Runtime.TencentESSBasicConfigFile != "" {
		paths = append(paths, filepath.Clean(config.Runtime.TencentESSBasicConfigFile))
	}
	for left := range paths {
		for right := left + 1; right < len(paths); right++ {
			if sameConfigurationPath(paths[left], paths[right]) {
				return Config{}, fmt.Errorf("local runtime configuration files must be distinct")
			}
		}
	}
	config.Runtime.ProcessAuthWindow, err = requiredProcessAuthWindow(
		getenv(processAuthWindowEnv),
	)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", processAuthWindowEnv, err)
	}
	return config, nil
}

func sameConfigurationPath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

// requiredProcessAuthWindow parses the mandatory bounded signature lifetime.
func requiredProcessAuthWindow(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("duration is required")
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < time.Second || duration > processauth.MaximumAuthenticationWindow {
		return 0, fmt.Errorf("duration must be between 1s and %s", processauth.MaximumAuthenticationWindow)
	}
	return duration, nil
}

// validateConfigurationFilePath accepts one bounded absolute private-file path.
func validateConfigurationFilePath(name, value string) error {
	if value == "" || len(value) > maximumConfigurationPathBytes || !filepath.IsAbs(value) ||
		strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("%s must be an absolute bounded file path", name)
	}
	return nil
}

// valueOrDefault returns a trimmed explicit value or its process default.
func valueOrDefault(value string, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

// durationOrDefault parses an optional bounded server timeout.
func durationOrDefault(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration")
	}
	if duration < minimumTimeout || duration > maximumTimeout {
		return 0, fmt.Errorf("duration must be between %s and %s", minimumTimeout, maximumTimeout)
	}
	return duration, nil
}

// validateListenAddress validates a concrete TCP host and numeric port.
func validateListenAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(port) == "" {
		return fmt.Errorf("%s must be a valid host:port address", listenAddressEnv)
	}
	if strings.ContainsAny(host, "\r\n\t") {
		return fmt.Errorf("%s contains invalid characters", listenAddressEnv)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("%s must contain a numeric port between 1 and 65535", listenAddressEnv)
	}
	return nil
}
