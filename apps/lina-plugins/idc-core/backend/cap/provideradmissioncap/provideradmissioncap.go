// Package provideradmissioncap shares the reviewed source-plugin admission wait.
// It registers no host service, keeps no state, and retries only a host refusal
// before the caller reports entering a guest or binding action.

package provideradmissioncap

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
)

const (
	// This public host code is emitted only when lock acquisition refused dispatch.
	admissionBusyCode = "PLUGIN_LIFECYCLE_ALREADY_RUNNING"
	admissionWait     = 125 * time.Millisecond
	admissionRetries  = 40
)

// IsBusy recognizes only the reviewed host admission refusal code,
// allowing safe diagnostics without exposing arbitrary underlying error text.
func IsBusy(err error) bool {
	metadata, ok := bizerr.As(err)
	return ok && metadata.RuntimeCode() == admissionBusyCode
}

// Wait retries only authoritative pre-dispatch refusals while entered is false.
// It returns the last refusal after five seconds, or the first other failure,
// cancellation or callback entry. It never owns or invokes an external request.
func Wait(ctx context.Context, attempt func() (bool, error)) error {
	for retry := 0; ; retry++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		entered, err := attempt()
		if entered || !IsBusy(err) || retry >= admissionRetries {
			return err
		}
		timer := time.NewTimer(admissionWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
