package operations

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// TimingSegment is one redacted part of a task execution.  Kind makes the
// slow part obvious to operators without exposing provider payloads.
type TimingSegment struct {
	Name      string
	Kind      string // local, queue, provider, database, other
	StartedAt time.Time
	EndedAt   time.Time
}

// TimingSummary is the bounded, structured timing projection used by the
// task center. Durations are milliseconds and are grouped by cause.
type TimingSummary struct {
	TotalMs      int64
	LocalMs      int64
	QueueMs      int64
	ProviderMs   int64
	DatabaseMs   int64
	OtherMs      int64
	SlowSegments []TimingSegmentSummary
}

type TimingSegmentSummary struct {
	Name       string
	Kind       string
	DurationMs int64
}

// SummarizeTiming validates and aggregates at most 32 segments. Segments are
// sorted by start time so callers receive deterministic output.
func SummarizeTiming(ctx context.Context, segments []TimingSegment, slowAfter time.Duration) (TimingSummary, error) {
	if ctx == nil {
		return TimingSummary{}, gerror.New("timing context is required")
	}
	if err := ctx.Err(); err != nil {
		return TimingSummary{}, err
	}
	if len(segments) == 0 || len(segments) > 32 {
		return TimingSummary{}, gerror.New("timing segment count is invalid")
	}
	if slowAfter <= 0 {
		slowAfter = 500 * time.Millisecond
	}
	ordered := append([]TimingSegment(nil), segments...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].StartedAt.Before(ordered[j].StartedAt) })
	result := TimingSummary{}
	for _, segment := range ordered {
		name := strings.TrimSpace(segment.Name)
		if name == "" || len(name) > 96 || segment.StartedAt.IsZero() || segment.EndedAt.IsZero() || segment.EndedAt.Before(segment.StartedAt) {
			return TimingSummary{}, gerror.New("timing segment is invalid")
		}
		for _, r := range name {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == ':') {
				return TimingSummary{}, gerror.New("timing segment name is invalid")
			}
		}
		kind := strings.ToLower(strings.TrimSpace(segment.Kind))
		if kind != "local" && kind != "queue" && kind != "provider" && kind != "database" && kind != "other" {
			return TimingSummary{}, gerror.New("timing segment kind is invalid")
		}
		duration := segment.EndedAt.Sub(segment.StartedAt)
		ms := duration.Milliseconds()
		result.TotalMs += ms
		switch kind {
		case "local":
			result.LocalMs += ms
		case "queue":
			result.QueueMs += ms
		case "provider":
			result.ProviderMs += ms
		case "database":
			result.DatabaseMs += ms
		default:
			result.OtherMs += ms
		}
		if duration >= slowAfter {
			result.SlowSegments = append(result.SlowSegments, TimingSegmentSummary{Name: name, Kind: kind, DurationMs: ms})
		}
	}
	return result, nil
}

// ClassifySignal derives a stable alert state from one redacted observation.
// Queue backlog uses count, inventory freshness uses age, and performance
// uses value (milliseconds). Adapter health uses count as failed checks.
func ClassifySignal(kind string, count, ageSeconds, valueMinor, warning, critical int64) (string, error) {
	if warning < 0 || critical <= warning {
		return "", gerror.New("signal thresholds are invalid")
	}
	if kind != "queue_backlog" && kind != "adapter_health" && kind != "inventory_freshness" && kind != "performance" {
		return "", gerror.New("operations signal kind is invalid")
	}
	if count < 0 || ageSeconds < 0 || valueMinor < 0 {
		return "", gerror.New("operations signal values are invalid")
	}
	value := valueMinor
	if kind == "queue_backlog" || kind == "adapter_health" {
		value = count
	} else if kind == "inventory_freshness" {
		value = ageSeconds
	}
	if value >= critical {
		return "critical", nil
	}
	if value >= warning {
		return "warning", nil
	}
	return "ok", nil
}
