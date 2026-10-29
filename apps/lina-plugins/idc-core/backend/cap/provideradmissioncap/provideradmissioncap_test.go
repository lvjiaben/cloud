// These tests verify the shared helper accepts only a typed, reviewed host code
// and stops on cancellation without retrying arbitrary textual errors.
package provideradmissioncap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

func TestAdmissionCategoryAndCancellation(t *testing.T) {
	typed := bizerr.NewCode(bizerr.MustDefine(admissionBusyCode, "private host explanation", gcode.CodeOperationFailed))
	if !IsBusy(typed) || IsBusy(errors.New(admissionBusyCode)) {
		t.Fatal("untrusted text accepted as host refusal")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	calls := 0
	if err := Wait(ctx, func() (bool, error) { calls++; return false, typed }); !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("cancellation calls=%d err=%v", calls, err)
	}
	calls = 0
	if err := Wait(context.Background(), func() (bool, error) { calls++; return true, typed }); err != typed || calls != 1 {
		t.Fatal("entered callback retried")
	}
}
