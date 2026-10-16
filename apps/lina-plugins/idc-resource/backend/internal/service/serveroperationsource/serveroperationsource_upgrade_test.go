// This file verifies paid upgrades cannot smuggle reinstall, reductions or
// unsupported disk/quota operations into a Provider resize command.

package serveroperationsource

import (
	"encoding/json"
	"testing"

	"lina-plugin-idc-resource/backend/internal/service/serverconfiguration"
)

func TestCompileUpgradeParametersRejectsReductionAndUnsupportedChanges(t *testing.T) {
	product := json.RawMessage(`{"region":"1","line":"2","cpu":2,"memory":2048,"system_disk":40,"data_disk":100,"bandwidth":10}`)
	current, err := serverconfiguration.Compile(product, json.RawMessage(`{"operating_system":"3"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, target, want string
		reject             bool
	}{
		{"cpu", "{\"operating_system\":\"3\",\"cpu\":4}", `{"cpu":4}`, false},
		{"memory", "{\"operating_system\":\"3\",\"memory_gib\":4}", `{"memory":4096}`, false},
		{"disk_increase", "{\"operating_system\":\"3\",\"system_disk\":80}", `{"system_disk_size":80}`, false},
		{"bandwidth", "{\"operating_system\":\"3\",\"bandwidth\":20}", `{"in_bw":20,"out_bw":20}`, false},
		{"same", "{\"operating_system\":\"3\"}", "", true},
		{"cpu_reduction", "{\"operating_system\":\"3\",\"cpu\":1}", "", true},
		{"disk_shrink", "{\"operating_system\":\"3\",\"system_disk\":20}", "", true},
		{"mixed_increase_and_reduction", "{\"operating_system\":\"3\",\"cpu\":4,\"memory_gib\":1}", "", true},
		{"reinstall", "{\"operating_system\":\"9\",\"cpu\":4}", "", true},
		{"data_disk_removed", "{\"operating_system\":\"3\",\"data_disk\":0,\"cpu\":4}", "", true},
		{"data_disk_mutated_without_identity", "{\"operating_system\":\"3\",\"data_disk\":200}", "", true},
		{"quota_change_not_resize", "{\"operating_system\":\"3\",\"snapshot_count\":5}", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			target, err := serverconfiguration.Compile(product, json.RawMessage(test.target))
			if err != nil {
				t.Fatal(err)
			}
			got, err := compileUpgradeParameters(current, target)
			if test.reject {
				if err == nil {
					t.Fatalf("unsafe resize accepted: %s", got)
				}
				return
			}
			if err != nil || string(got) != test.want {
				t.Fatalf("got %s, %v; want %s", got, err, test.want)
			}
		})
	}
}
