// This file proves that runtime configuration depends only on files and values
// owned by the current installation.

package app

import (
	"reflect"
	"testing"
	"time"
)

func TestRuntimeConfigurationHasOnlyInstallationLocalDependencies(t *testing.T) {
	typeOfRuntime := reflect.TypeOf(RuntimeConfig{})
	if typeOfRuntime.NumField() != 7 {
		t.Fatalf("runtime dependency field count=%d want=7", typeOfRuntime.NumField())
	}
	want := map[string]reflect.Type{
		"DatabaseDSNFile":             reflect.TypeOf(""),
		"InstallationIDFile":          reflect.TypeOf(""),
		"ProcessAuthKeyringFile":      reflect.TypeOf(""),
		"SupplierIdentityKeyringFile": reflect.TypeOf(""),
		"CallbackEvidenceKeyringFile": reflect.TypeOf(""),
		"TencentESSBasicConfigFile":   reflect.TypeOf(""),
		"ProcessAuthWindow":           reflect.TypeOf(time.Duration(0)),
	}
	found := make(map[string]bool, len(want))
	for index := 0; index < typeOfRuntime.NumField(); index++ {
		field := typeOfRuntime.Field(index)
		expectedType, ok := want[field.Name]
		if !ok || field.Type != expectedType {
			t.Fatalf("unexpected runtime dependency field %s %s", field.Name, field.Type)
		}
		found[field.Name] = true
	}
	for name := range want {
		if !found[name] {
			t.Fatalf("missing installation-local runtime dependency %s", name)
		}
	}
}
