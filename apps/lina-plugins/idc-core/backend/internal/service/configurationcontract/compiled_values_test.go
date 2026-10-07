package configurationcontract

import (
	"bytes"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestDecodeCompiledValuesAcceptsOnlyCompilerShapes(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "字段", EnUS: "Field"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "name", Type: capmodel.ConfigurationFieldSingleLine, Label: label},
		{Key: "count", Type: capmodel.ConfigurationFieldInteger, Label: label},
		{Key: "enabled", Type: capmodel.ConfigurationFieldBoolean, Label: label},
		{Key: "files", Type: capmodel.ConfigurationFieldFile, Label: label, UploadPolicy: "files-v1"},
	}, UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{{ID: "files-v1", MaximumFiles: 2, MaximumFileBytes: 1024, MaximumTotalBytes: 2048, AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"}}}}
	name := "sample"
	count := int64(2)
	enabled := true
	values := []ConfigurationValue{{FieldKey: "name", TextValue: &name}, {FieldKey: "count", IntegerValue: &count}, {FieldKey: "enabled", BooleanValue: &enabled}, {FieldKey: "files", FileTokens: []string{"CFU_0123456789ABCDEF0123456789ABCDEF"}}}
	compiled, err := Compile(schema, values, SurfaceCustomer, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCompiledValues(schema, compiled.ValuesJSON)
	if err != nil {
		t.Fatal(err)
	}
	recompiled, err := Compile(schema, decoded, SurfaceCustomer, nil)
	if err != nil || !bytes.Equal(compiled.ValuesJSON, recompiled.ValuesJSON) {
		t.Fatalf("recompile=%s err=%v", recompiled.ValuesJSON, err)
	}
	for _, invalid := range [][]byte{[]byte(`{"unknown":true}`), []byte(`{"count":"2"}`), append(compiled.ValuesJSON, []byte(` {}`)...)} {
		if result, decodeErr := DecodeCompiledValues(schema, invalid); decodeErr == nil || result != nil {
			t.Fatalf("invalid compiler document accepted: %s", invalid)
		}
	}
}
