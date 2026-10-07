package idccore

import (
	"bytes"
	"io/fs"
	"regexp"
	"testing"
)

func TestAcceptanceMockCustomersUsePersistedCustomerNumbers(t *testing.T) {
	data, err := fs.ReadFile(
		EmbeddedFiles,
		"manifest/sql/mock-data/001-idc-core-acceptance-demo.sql",
	)
	if err != nil {
		t.Fatalf("read IDC acceptance mock data: %v", err)
	}

	for _, legacy := range [][]byte{
		[]byte("demo_cus_ind_verified"),
		[]byte("demo_cus_ind_pending"),
		[]byte("demo_cus_ind_restricted"),
		[]byte("demo_cus_ind_disabled"),
		[]byte("demo_cus_ent_verified"),
		[]byte("demo_cus_ent_pending"),
	} {
		if bytes.Contains(data, legacy) {
			t.Fatalf("mock customer number bypasses the production format: %s", legacy)
		}
	}

	validCustomerNo := regexp.MustCompile(`CUS_[0-9a-f]{32}`)
	if got := len(validCustomerNo.FindAll(data, -1)); got < 6 {
		t.Fatalf("expected at least six persisted-format mock customer numbers, got %d", got)
	}
}

func TestAcceptanceMockProductVersionsOwnInventoryRows(t *testing.T) {
	data, err := fs.ReadFile(
		EmbeddedFiles,
		"manifest/sql/mock-data/001-idc-core-acceptance-demo.sql",
	)
	if err != nil {
		t.Fatalf("read IDC acceptance mock data: %v", err)
	}

	for _, versionNo := range [][]byte{
		[]byte("ver_00000000000000000000000000000021"),
		[]byte("ver_00000000000000000000000000000022"),
		[]byte("ver_00000000000000000000000000000023"),
		[]byte("ver_00000000000000000000000000000024"),
	} {
		if bytes.Count(data, versionNo) < 2 {
			t.Fatalf("mock product version has no inventory row: %s", versionNo)
		}
	}
}
