package supportdesk

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAppendStaffAccessHonorsHostDataScope(t *testing.T) {
	tests := []struct {
		name              string
		actor             actor
		wantDepartmentSQL bool
		wantQueueSQL      bool
		wantArgs          []any
	}{
		{
			name:              "current department uses live organization mapping",
			wantDepartmentSQL: true,
			actor:             actor{UserID: 17, DataScope: 3, Leader: true, Departments: []string{"DEP_support"}},
			wantArgs:          []any{17, 17, "DEP_support", "DEP_support"},
		},
		{
			name:              "self scope requires live department membership",
			wantDepartmentSQL: true,
			actor:             actor{UserID: 18, DataScope: 4, Leader: true, Departments: []string{"DEP_support"}},
			wantArgs:          []any{18, 18, "DEP_support"},
		},
		{
			name:              "tenant scope may use proven support membership and leader queue",
			actor:             actor{UserID: 19, DataScope: 2, Leader: true, Departments: []string{"DEP_support"}},
			wantDepartmentSQL: true,
			wantQueueSQL:      true,
			wantArgs:          []any{19, 19, "DEP_support", "DEP_support"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var where strings.Builder
			args := make([]any, 0)
			appendStaffAccess(&where, &args, &test.actor, "t")
			query := where.String()
			if strings.Contains(query, "department_no IN") != test.wantDepartmentSQL {
				t.Fatalf("department scope mismatch: %s", query)
			}
			if strings.Contains(query, "department_no IS NULL") != test.wantQueueSQL {
				t.Fatalf("public queue scope mismatch: %s", query)
			}
			if !reflect.DeepEqual(args, test.wantArgs) {
				t.Fatalf("scope args = %#v, want %#v", args, test.wantArgs)
			}
		})
	}
}

func TestCurrentSLARiskUsesStableAuthorityStates(t *testing.T) {
	now := time.Date(2026, time.August, 15, 8, 0, 0, 0, time.UTC)
	firstDue := now.Add(30 * time.Minute)
	resolutionDue := now.Add(2 * time.Hour)
	base := slaRiskRow{
		NearDueMinutes:     []int{60, 15},
		FirstResponseDueAt: &firstDue,
		ResolutionDueAt:    &resolutionDue,
	}
	if got := currentSLARisk(now, base, false, false); got != SLARiskAtRisk {
		t.Fatalf("near-due risk = %q, want %q", got, SLARiskAtRisk)
	}
	responded := now.Add(-time.Minute)
	base.FirstRespondedAt = &responded
	if got := currentSLARisk(now, base, false, false); got != SLARiskNone {
		t.Fatalf("completed first-response timer risk = %q, want %q", got, SLARiskNone)
	}
	if got := currentSLARisk(now, base, true, false); got != SLARiskBreached {
		t.Fatalf("breached risk = %q, want %q", got, SLARiskBreached)
	}
	base.ResolutionTimerIsPaused = true
	base.ResolutionDueAt = &firstDue
	if got := currentSLARisk(now, base, false, false); got != SLARiskNone {
		t.Fatalf("paused resolution risk = %q, want %q", got, SLARiskNone)
	}
}

func TestTimerBreachedRequiresStrictlyPastDueAndRunning(t *testing.T) {
	now := time.Date(2026, time.August, 15, 8, 0, 0, 0, time.UTC)
	due := now
	if timerBreached(now, &due, nil, false) {
		t.Fatal("timer breached exactly at due time")
	}
	past := now.Add(-time.Nanosecond)
	if !timerBreached(now, &past, nil, false) {
		t.Fatal("running timer did not breach after due time")
	}
	completed := past
	if timerBreached(now, &past, &completed, false) {
		t.Fatal("completed timer breached")
	}
	if timerBreached(now, &past, nil, true) {
		t.Fatal("paused timer breached")
	}
}
