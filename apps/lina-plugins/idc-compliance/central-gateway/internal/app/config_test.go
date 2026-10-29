// This file verifies the local gateway's required installation-owned runtime
// files and their cross-platform path isolation.

package app

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigRequiresLocalRuntimeFiles(t *testing.T) {
	root := t.TempDir()
	values := map[string]string{
		databaseDSNFileEnv:             filepath.Join(root, "database.dsn"),
		installationIDFileEnv:          filepath.Join(root, "installation-id"),
		processAuthKeyringFileEnv:      filepath.Join(root, "process-auth.json"),
		supplierIdentityKeyringFileEnv: filepath.Join(root, "supplier-identity.json"),
		callbackEvidenceKeyringFileEnv: filepath.Join(root, "callback-evidence.json"),
		processAuthWindowEnv:           "60s",
	}
	config, err := LoadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if config.ListenAddress != defaultListenAddress || config.ReadHeaderTimeout != defaultHeaderTimeout ||
		config.ShutdownTimeout != defaultShutdownTimeout || config.MaxHeaderBytes != maximumHeaderBytes ||
		config.Runtime.DatabaseDSNFile != values[databaseDSNFileEnv] ||
		config.Runtime.InstallationIDFile != values[installationIDFileEnv] ||
		config.Runtime.SupplierIdentityKeyringFile != values[supplierIdentityKeyringFileEnv] ||
		config.Runtime.CallbackEvidenceKeyringFile != values[callbackEvidenceKeyringFileEnv] ||
		config.Runtime.ProcessAuthWindow != time.Minute {
		t.Fatalf("unexpected config: %+v", config)
	}
	for _, required := range []string{
		databaseDSNFileEnv,
		installationIDFileEnv,
		processAuthKeyringFileEnv,
		supplierIdentityKeyringFileEnv,
		callbackEvidenceKeyringFileEnv,
	} {
		missing := map[string]string{}
		for key, value := range values {
			missing[key] = value
		}
		delete(missing, required)
		if _, err = LoadConfig(func(key string) string { return missing[key] }); err == nil {
			t.Fatalf("missing local runtime file %s was accepted", required)
		}
	}
}

func TestLoadConfigAcceptsBoundedOverrides(t *testing.T) {
	root := t.TempDir()
	values := map[string]string{
		listenAddressEnv:               "0.0.0.0:9443",
		readHeaderTimeoutEnv:           "2s",
		readTimeoutEnv:                 "3s",
		writeTimeoutEnv:                "4s",
		idleTimeoutEnv:                 "5s",
		shutdownTimeoutEnv:             "6s",
		databaseDSNFileEnv:             filepath.Join(root, "database.dsn"),
		installationIDFileEnv:          filepath.Join(root, "installation-id"),
		processAuthKeyringFileEnv:      filepath.Join(root, "process-auth.json"),
		supplierIdentityKeyringFileEnv: filepath.Join(root, "supplier-identity.json"),
		callbackEvidenceKeyringFileEnv: filepath.Join(root, "callback-evidence.json"),
		processAuthWindowEnv:           "45s",
	}
	config, err := LoadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if config.ListenAddress != "0.0.0.0:9443" || config.ReadHeaderTimeout != 2*time.Second ||
		config.ShutdownTimeout != 6*time.Second {
		t.Fatalf("unexpected overrides: %+v", config)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	root := t.TempDir()
	base := map[string]string{
		databaseDSNFileEnv:             filepath.Join(root, "database.dsn"),
		installationIDFileEnv:          filepath.Join(root, "installation-id"),
		processAuthKeyringFileEnv:      filepath.Join(root, "process-auth.json"),
		supplierIdentityKeyringFileEnv: filepath.Join(root, "supplier-identity.json"),
		callbackEvidenceKeyringFileEnv: filepath.Join(root, "callback-evidence.json"),
		processAuthWindowEnv:           "60s",
	}
	tests := []map[string]string{
		{listenAddressEnv: "127.0.0.1"},
		{listenAddressEnv: "127.0.0.1:70000"},
		{readTimeoutEnv: "500ms"},
		{writeTimeoutEnv: "not-a-duration"},
		{processAuthKeyringFileEnv: base[databaseDSNFileEnv]},
		{supplierIdentityKeyringFileEnv: base[databaseDSNFileEnv]},
		{installationIDFileEnv: base[databaseDSNFileEnv]},
		{callbackEvidenceKeyringFileEnv: base[databaseDSNFileEnv]},
		{processAuthWindowEnv: ""},
		{processAuthWindowEnv: "61s"},
	}
	for index, overrides := range tests {
		values := map[string]string{}
		for key, value := range base {
			values[key] = value
		}
		for key, value := range overrides {
			values[key] = value
		}
		if _, err := LoadConfig(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("case %d expected an error", index)
		}
	}
	if runtime.GOOS == "windows" {
		values := map[string]string{}
		for key, value := range base {
			values[key] = value
		}
		values[supplierIdentityKeyringFileEnv] = strings.ToUpper(base[databaseDSNFileEnv])
		if _, err := LoadConfig(func(key string) string { return values[key] }); err == nil {
			t.Fatal("case-variant duplicate runtime path was accepted")
		}
	}
}
