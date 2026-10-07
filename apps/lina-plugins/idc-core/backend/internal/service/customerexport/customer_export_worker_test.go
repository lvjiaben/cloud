package customerexport

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestBuildArchiveIsDeterministicAndPrivatePayloadIsReadable(t *testing.T) {
	generatedAt := time.Date(2026, time.August, 18, 12, 34, 56, 123000000, time.UTC)
	document := &exportDocument{
		SchemaVersion: 1,
		GeneratedAt:   generatedAt,
		Profile: map[string]any{
			"customer_no":  "CUS_0123456789abcdef0123456789abcdef",
			"display_name": "Example customer",
		},
		Contacts: []map[string]any{{"contact_no": "CON_0123456789abcdef0123456789abcdef"}},
	}
	first, err := buildArchive(document)
	if err != nil {
		t.Fatalf("build first archive failed: %v", err)
	}
	second, err := buildArchive(document)
	if err != nil {
		t.Fatalf("build second archive failed: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same export request produced different archive bytes")
	}
	reader, err := zip.NewReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatalf("open export archive failed: %v", err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "customer-data.json" ||
		!reader.File[0].Modified.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("unexpected export archive entries: %+v", reader.File)
	}
	file, err := reader.File[0].Open()
	if err != nil {
		t.Fatalf("open export document failed: %v", err)
	}
	defer file.Close()
	payload, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read export document failed: %v", err)
	}
	var decoded exportDocument
	if err = json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode export document failed: %v", err)
	}
	if decoded.SchemaVersion != 1 || !decoded.GeneratedAt.Equal(generatedAt) ||
		decoded.Profile["customer_no"] != document.Profile["customer_no"] {
		t.Fatalf("unexpected export document: %+v", decoded)
	}
}
