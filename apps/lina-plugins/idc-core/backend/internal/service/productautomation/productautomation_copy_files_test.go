// This file verifies copied upload evidence respects aggregate field limits,
// including exact boundaries and overflow-resistant byte accounting.

package productautomation

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestCopiedFileAggregateLimits(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{
		UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{{ID: "proof", MaximumFiles: 2, MaximumTotalBytes: 10}},
		Fields:         []*capmodel.ConfigurationFieldDeclaration{{Key: "document", Type: capmodel.ConfigurationFieldFile, UploadPolicy: "proof"}},
	}
	for _, test := range []struct {
		name  string
		sizes []int64
		valid bool
	}{
		{"exact limit", []int64{4, 6}, true},
		{"too many files", []int64{1, 1, 1}, false},
		{"too many bytes", []int64{6, 5}, false},
		{"overflow", []int64{1, 9223372036854775807}, false},
		{"empty evidence", []int64{0}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := make([]*managementFileRow, 0, len(test.sizes))
			for _, size := range test.sizes {
				rows = append(rows, &managementFileRow{FieldKey: "document", ActualSize: size})
			}
			if err := validateCopiedFileTotals(schema, rows); (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}
