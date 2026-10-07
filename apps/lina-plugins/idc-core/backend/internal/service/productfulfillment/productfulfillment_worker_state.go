package productfulfillment

import "lina-plugin-idc-core/backend/cap/productfulfillmentcap"

type workerDecision struct {
	TaskStatus, IntentStatus, PhaseCode string
	ScheduleNext                        bool
}

func decideWorkerResult(previousTaskStatus, previousPhase string, action productfulfillmentcap.Action, result *productfulfillmentcap.Result, invokeFailed, providerBoundaryCrossed, takeoverRequested, cancelRequested bool, queryAttempts, submitAttempts int) workerDecision {
	manual := workerDecision{TaskStatus: "manual_takeover", IntentStatus: "manual_pending", PhaseCode: "frozen_unknown"}
	cancellationAttention := workerDecision{TaskStatus: "cancellation_attention", IntentStatus: "manual_pending", PhaseCode: "frozen_unknown"}
	if invokeFailed {
		if takeoverRequested {
			return manual
		}
		// Failures while preparing the governed runtime context are known to
		// have happened before the provider boundary. They may consume the
		// bounded local attempt, but they must never be recorded as an
		// uncertain supplier write. A failed submit preparation goes back
		// through query-first before any later submit is considered.
		if !providerBoundaryCrossed {
			if cancelRequested && action == productfulfillmentcap.ActionSubmit {
				return workerDecision{TaskStatus: "canceled", IntentStatus: "canceled", PhaseCode: "frozen_unknown"}
			}
			if queryAttempts >= maximumQueryAttempts {
				if cancelRequested {
					return cancellationAttention
				}
				return workerDecision{TaskStatus: "failed", IntentStatus: "manual_pending", PhaseCode: "pre_submit_query"}
			}
			return workerDecision{TaskStatus: "retry_wait", IntentStatus: "processing", PhaseCode: "pre_submit_query", ScheduleNext: true}
		}
		// A submit transport failure may mean the supplier accepted the write.
		// It is never safe to submit again automatically.
		if action == productfulfillmentcap.ActionSubmit {
			if cancelRequested {
				return cancellationAttention
			}
			return workerDecision{TaskStatus: "result_unknown", IntentStatus: "processing", PhaseCode: "frozen_unknown"}
		}
		if previousTaskStatus == "accepted" || previousTaskStatus == "running" {
			return workerDecision{TaskStatus: previousTaskStatus, IntentStatus: "processing", PhaseCode: "post_submit_query", ScheduleNext: queryAttempts < maximumQueryAttempts}
		}
		if queryAttempts >= maximumQueryAttempts {
			if cancelRequested {
				return cancellationAttention
			}
			return workerDecision{TaskStatus: "failed", IntentStatus: "manual_pending", PhaseCode: previousPhase}
		}
		return workerDecision{TaskStatus: "retry_wait", IntentStatus: "processing", PhaseCode: previousPhase, ScheduleNext: true}
	}
	if result == nil {
		if takeoverRequested {
			return manual
		}
		if cancelRequested {
			return cancellationAttention
		}
		return workerDecision{TaskStatus: "result_unknown", IntentStatus: "processing", PhaseCode: "frozen_unknown"}
	}
	terminal := func(taskStatus, intentStatus string) workerDecision {
		return workerDecision{TaskStatus: taskStatus, IntentStatus: intentStatus, PhaseCode: "post_submit_query"}
	}
	switch result.Status {
	case productfulfillmentcap.StatusSucceeded:
		return terminal("succeeded", "completed")
	case productfulfillmentcap.StatusFailed:
		return terminal("failed", "incomplete")
	case productfulfillmentcap.StatusResultUnknown:
		if takeoverRequested {
			return manual
		}
		if cancelRequested {
			return cancellationAttention
		}
		return workerDecision{TaskStatus: "result_unknown", IntentStatus: "processing", PhaseCode: "frozen_unknown"}
	}
	if takeoverRequested {
		return manual
	}
	if cancelRequested {
		return decideCancellationResult(action, result.Status)
	}
	switch result.Status {
	case productfulfillmentcap.StatusNotFound:
		if previousPhase != "pre_submit_query" {
			return workerDecision{TaskStatus: "result_unknown", IntentStatus: "processing", PhaseCode: "frozen_unknown"}
		}
		if submitAttempts >= maximumSubmitAttempts {
			return workerDecision{TaskStatus: "failed", IntentStatus: "manual_pending", PhaseCode: previousPhase}
		}
		return workerDecision{TaskStatus: "pending_submit", IntentStatus: "processing", PhaseCode: "submit_allowed", ScheduleNext: true}
	case productfulfillmentcap.StatusNotAccepted:
		if submitAttempts >= maximumSubmitAttempts || queryAttempts >= maximumQueryAttempts {
			return workerDecision{TaskStatus: "failed", IntentStatus: "manual_pending", PhaseCode: "pre_submit_query"}
		}
		return workerDecision{TaskStatus: "retry_wait", IntentStatus: "processing", PhaseCode: "pre_submit_query", ScheduleNext: true}
	case productfulfillmentcap.StatusAccepted:
		return workerDecision{TaskStatus: "accepted", IntentStatus: "processing", PhaseCode: "post_submit_query", ScheduleNext: queryAttempts < maximumQueryAttempts}
	case productfulfillmentcap.StatusRunning:
		return workerDecision{TaskStatus: "running", IntentStatus: "processing", PhaseCode: "post_submit_query", ScheduleNext: queryAttempts < maximumQueryAttempts}
	default:
		return workerDecision{TaskStatus: "result_unknown", IntentStatus: "processing", PhaseCode: "frozen_unknown"}
	}
}

func decideCancellationResult(action productfulfillmentcap.Action, status productfulfillmentcap.ResultStatus) workerDecision {
	if (action == productfulfillmentcap.ActionQuery && status == productfulfillmentcap.StatusNotFound) ||
		(action == productfulfillmentcap.ActionSubmit && status == productfulfillmentcap.StatusNotAccepted) {
		return workerDecision{TaskStatus: "canceled", IntentStatus: "canceled", PhaseCode: "frozen_unknown"}
	}
	return workerDecision{TaskStatus: "cancellation_attention", IntentStatus: "manual_pending", PhaseCode: "frozen_unknown"}
}
