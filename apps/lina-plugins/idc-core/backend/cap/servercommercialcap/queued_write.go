// Queued writes retain a core-owned execution fence after committing admission,
// so provider network calls do not need to hold a database transaction open.
package servercommercialcap

import (
	"context"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type QueuedWrite struct {
	OperationNo, TaskNo, EntitlementNo, CustomerNo, RequestDigest string
	ExpectedEntitlementVersion                                    int64
	Action                                                        serveroperationresourcecap.Action
}

type QueuedWriteCompletion struct {
	TaskNo, EntitlementNo, CustomerNo, EvidenceDigest string
	Status                                            CompletionStatus
}

// QueuedWriteAccess is source-only and applies to ordinary native operations.
// Commercial tasks retain their existing immutable business-operation fence.
type QueuedWriteAccess interface {
	// ListQueuedWrites rotates a bounded batch of unresolved fences. The resource
	// owner batch-loads their actual task receipts and submits verified outcomes;
	// an unresolved task is never inferred to have failed merely because it is old.
	ListQueuedWrites(context.Context, int) ([]QueuedWrite, error)
	// HoldQueuedWrite requires the shared core/resource transaction after the
	// resource owner validates and durably claims the frozen task's actual lease.
	// An identical executing hold does not grant permission to replay a provider
	// call. Unknown and terminal tasks cannot acquire another execution hold.
	HoldQueuedWrite(context.Context, QueuedWrite) error
	// CompleteQueuedWrite consumes a receipt verified by the resource reconciler.
	// Unknown results retain the fence; only a confirmed terminal result releases it.
	CompleteQueuedWrite(context.Context, QueuedWriteCompletion) error
}
