package principal

import (
	"context"
	"errors"
	"sync"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type principalTestAuditWriter struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
	err         error
}

func (w *principalTestAuditWriter) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.transitions = append(w.transitions, input)
	return nil
}

func (*principalTestAuditWriter) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch audit call")
}

func (w *principalTestAuditWriter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.transitions)
}

var _ idccap.BusinessAuditWriter = (*principalTestAuditWriter)(nil)
