package customeroperations

import (
	"context"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

type nativeObjectOwner struct {
	servercommercialcap.Service
	blocked  bool
	hold     func(context.Context, servercommercialcap.QueuedWrite) error
	receipts []servercommercialcap.QueuedWriteCompletion
	writes   []servercommercialcap.QueuedWrite
}

func (o *nativeObjectOwner) CheckRestore(context.Context, servercommercialcap.ObjectRequest) error {
	if o.blocked {
		return ErrConflict
	}
	return nil
}

func (o *nativeObjectOwner) CheckQueuedRestore(ctx context.Context, input servercommercialcap.ObjectRequest) error {
	return o.CheckRestore(ctx, input)
}

func (o *nativeObjectOwner) HoldQueuedWrite(ctx context.Context, input servercommercialcap.QueuedWrite) error {
	if o.hold != nil {
		return o.hold(ctx, input)
	}
	return nil
}
func (o *nativeObjectOwner) CompleteQueuedWrite(_ context.Context, input servercommercialcap.QueuedWriteCompletion) error {
	o.receipts = append(o.receipts, input)
	return nil
}
func (o *nativeObjectOwner) ListQueuedWrites(context.Context, int) ([]servercommercialcap.QueuedWrite, error) {
	return o.writes, nil
}
