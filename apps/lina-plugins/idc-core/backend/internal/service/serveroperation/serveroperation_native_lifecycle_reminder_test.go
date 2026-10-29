// Reminder amount formatting preserves accounting precision without floats.

package serveroperation

import "testing"

func TestAccessoryRenewalReminderCurrencyPrecision(t *testing.T) {
	for _, check := range []struct {
		code, want string
		minor      int64
	}{
		{"CNY", "1.05", 105}, {"USD", "0.01", 1}, {"JPY", "105", 105}, {"KWD", "0.105", 105},
	} {
		value, err := formatAccessoryRenewalAmount(check.minor, check.code)
		if err != nil || value != check.want {
			t.Fatalf("%s: %q want %q err=%v", check.code, value, check.want, err)
		}
	}
	if _, err := formatAccessoryRenewalAmount(-1, "CNY"); err == nil {
		t.Fatal("negative renewal amount was formatted")
	}
}
