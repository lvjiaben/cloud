// These tests reject ambiguous inputs and keep optional reinstall choices unset.
package customeroperations

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"testing"
)

func TestNativeActionParameters(t *testing.T) {
	for _, test := range []struct{ action, raw, want string }{
		{"reinstall", `{"os":101}`, `{"os":"101"}`},
		{"reinstall", `{"os":"101","port":"1234","format_data_disk":1,"system_disk_size":40}`, `{"format_data_disk":1,"os":"101","port":"1234","system_disk_size":"40"}`},
		{"reboot", `{}`, `{"mode":"graceful"}`},
		{"stop", `{"mode":"hard"}`, `{"mode":"hard"}`},
		{"nat_create", `{"name":"web","externalPort":"8080","internalPort":80,"protocol":"tcp"}`, `{"externalPort":8080,"internalPort":80,"name":"web","protocol":"tcp"}`},
	} {
		actual, err := normalizeParameters(serveroperationresourcecap.Action(test.action), json.RawMessage(test.raw))
		if err != nil || string(actual) != test.want {
			t.Fatalf("%s: %s %v", test.action, actual, err)
		}
	}
	for _, test := range []struct{ action, raw string }{
		{"reinstall", `{"os":"101","os":"102"}`}, {"reinstall", `{"os":"101","password":"secret"}`},
		{"reinstall", `{"os":"101","port":"auto"}`}, {"reinstall", `{"os":0}`}, {"reinstall", `{"os":101.1}`},
		{"reinstall", `{"os":101,"format_data_disk":true}`}, {"reinstall", `{"os":101,"system_disk_size":0}`},
		{"console", `{"url":"https://attacker.invalid"}`}, {"reset_password", `{"password":"secret"}`},
		{"nat_delete", `{"objectNo":101}`}, {"nat_create", `{"name":"web","externalPort":65536,"internalPort":80,"protocol":"tcp"}`},
		{"start", `[]`}, {"start", `{} {}`}, {"start", `{"x":null}`}, {"resize", `{"cpu":4}`},
	} {
		if _, err := normalizeParameters(serveroperationresourcecap.Action(test.action), json.RawMessage(test.raw)); err == nil {
			t.Fatalf("unsafe %s accepted: %s", test.action, test.raw)
		}
	}
}

func TestNativeGuestInformationWhitelist(t *testing.T) {
	info := observationInformation(json.RawMessage(`{"hostname":"cloud-1","os_name":"Debian 12","login_username":"root","port":1234,"cpu_cores":2,"memory_mib":2048,"rescue":false,"username":"provider-account","password":"secret","boot_order":"cd"}`))
	if info.Hostname != "cloud-1" || info.LoginUsername != "root" || info.Port == nil || *info.Port != 1234 || info.Rescue == nil || *info.Rescue {
		t.Fatalf("confirmed information: %+v", info)
	}
	info = observationInformation(json.RawMessage(`{"hostname":"<script>","login_username":"root\nattack","port":0,"cpu_cores":-1,"memory_mib":"2048","rescue":null}`))
	if info.Hostname != "" || info.LoginUsername != "" || info.Port != nil || info.CPUCores != nil || info.MemoryMiB != nil || info.Rescue != nil {
		t.Fatalf("invalid facts became confirmed: %+v", info)
	}
}
