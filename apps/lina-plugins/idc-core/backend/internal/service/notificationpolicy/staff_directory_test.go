package notificationpolicy

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/statusflag"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type staffDirectoryUserStub struct {
	usercap.Service
	items map[usercap.UserID]*usercap.UserInfo
	err   error
	calls int
}

func (s *staffDirectoryUserStub) BatchGet(
	_ context.Context,
	ids []usercap.UserID,
) (*capmodel.BatchResult[*usercap.UserInfo, usercap.UserID], error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	items := make(map[usercap.UserID]*usercap.UserInfo, len(ids))
	missing := make([]usercap.UserID, 0)
	for _, id := range ids {
		if item := s.items[id]; item != nil {
			items[id] = item
		} else {
			missing = append(missing, id)
		}
	}
	return &capmodel.BatchResult[*usercap.UserInfo, usercap.UserID]{Items: items, MissingIDs: missing}, nil
}

type staffDirectoryPrincipalStub struct {
	states map[int]principal.StaffDirectoryState
	err    error
	calls  int
}

func (s *staffDirectoryPrincipalStub) ResolveBatch(
	_ context.Context,
	_ []int,
) (map[int]principal.StaffDirectoryState, error) {
	s.calls++
	return s.states, s.err
}

func TestStaffDirectoryResolvesOnlyActiveVersionedStaff(t *testing.T) {
	const callerID = 90
	users := &staffDirectoryUserStub{items: map[usercap.UserID]*usercap.UserInfo{}}
	states := make(map[int]principal.StaffDirectoryState)
	requests := make([]notificationcap.StaffDirectoryRequest, 0)
	add := func(userID int, expectedVersion int64, tenant string, status statusflag.Enabled, state principal.StaffDirectoryState) {
		id := usercap.UserID(strconv.Itoa(userID))
		users.items[id] = &usercap.UserInfo{ID: id, TenantID: capmodel.DomainID(tenant), Status: status}
		states[userID] = state
		requests = append(requests, notificationcap.StaffDirectoryRequest{HostUserID: userID, ExpectedVersion: expectedVersion})
	}
	add(11, 2, "0", statusflag.EnabledValue, principal.StaffDirectoryState{UserID: 11, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 2})
	add(12, 2, "9", statusflag.EnabledValue, principal.StaffDirectoryState{UserID: 12, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 2})
	add(13, 2, "0", statusflag.Disabled, principal.StaffDirectoryState{UserID: 13, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 2})
	add(14, 4, "0", statusflag.EnabledValue, principal.StaffDirectoryState{UserID: 14, Code: principal.AudienceStaff, Status: principal.AudienceStatusRevoked, Version: 4})
	add(15, 5, "0", statusflag.EnabledValue, principal.StaffDirectoryState{UserID: 15, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, Version: 5})
	add(16, 6, "0", statusflag.EnabledValue, principal.StaffDirectoryState{UserID: 16, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 6, HasCustomerBinding: true})
	add(19, 2, "0", statusflag.EnabledValue, principal.StaffDirectoryState{UserID: 19, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 3})
	requests = append(requests,
		notificationcap.StaffDirectoryRequest{HostUserID: 17, ExpectedVersion: 7},
		notificationcap.StaffDirectoryRequest{HostUserID: 18, ExpectedVersion: 8},
	)
	states[17] = principal.StaffDirectoryState{UserID: 17, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 7}
	id18 := usercap.UserID("18")
	users.items[id18] = &usercap.UserInfo{ID: id18, TenantID: capmodel.DomainID("0"), Status: statusflag.EnabledValue}

	directory, err := NewStaffDirectory(
		currentStaffBizCtxStub{current: bizctxcap.CurrentContext{UserID: callerID, TenantID: 0, DataScope: 2}},
		users,
		&currentStaffPrincipalStub{audience: &principal.Audience{UserID: callerID, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 1}},
		&staffDirectoryPrincipalStub{states: states},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := directory.ResolveActiveBatch(context.Background(), requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != len(requests) || !result[0].Active || result[0].CurrentVersion != 2 {
		t.Fatalf("unexpected active result: %+v", result)
	}
	for _, index := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		if result[index].Active {
			t.Fatalf("unsafe target %d was active: %+v", index, result[index])
		}
	}
	for index, want := range map[int]int64{1: 2, 2: 2, 3: 4, 4: 0, 5: 0, 6: 3, 7: 7, 8: 0} {
		if result[index].CurrentVersion != want {
			t.Fatalf("target %d current version=%d want=%d", index, result[index].CurrentVersion, want)
		}
	}
}

func TestStaffDirectoryUsesOneBatchReadAtLimit(t *testing.T) {
	const callerID = 90
	users := &staffDirectoryUserStub{items: make(map[usercap.UserID]*usercap.UserInfo, notificationcap.MaximumStaffDirectoryBatch)}
	principals := &staffDirectoryPrincipalStub{states: make(map[int]principal.StaffDirectoryState, notificationcap.MaximumStaffDirectoryBatch)}
	requests := make([]notificationcap.StaffDirectoryRequest, 0, notificationcap.MaximumStaffDirectoryBatch)
	for index := range notificationcap.MaximumStaffDirectoryBatch {
		userID := 1_000 + index
		capabilityID := usercap.UserID(strconv.Itoa(userID))
		users.items[capabilityID] = &usercap.UserInfo{ID: capabilityID, TenantID: "0", Status: statusflag.EnabledValue}
		principals.states[userID] = principal.StaffDirectoryState{
			UserID: userID, Code: principal.AudienceStaff,
			Status: principal.AudienceStatusActive, Version: 1,
		}
		requests = append(requests, notificationcap.StaffDirectoryRequest{HostUserID: userID, ExpectedVersion: 1})
	}
	directory, err := NewStaffDirectory(
		testBizCtx(bizctxcap.CurrentContext{UserID: callerID, TenantID: 0, DataScope: 2}),
		users,
		&currentStaffPrincipalStub{audience: &principal.Audience{
			UserID: callerID, Code: principal.AudienceStaff,
			Status: principal.AudienceStatusActive, Version: 1,
		}},
		principals,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := directory.ResolveActiveBatch(context.Background(), requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != len(requests) {
		t.Fatalf("result count=%d want=%d", len(result), len(requests))
	}
	for index, item := range result {
		if !item.Active || item.CurrentVersion != 1 {
			t.Fatalf("result %d=%+v", index, item)
		}
	}
	if users.calls != 1 || principals.calls != 1 {
		t.Fatalf("batch readers called users=%d principals=%d", users.calls, principals.calls)
	}
}

func TestStaffDirectoryRejectsInvalidCallerBeforeBatchReads(t *testing.T) {
	tests := []struct {
		name    string
		current bizctxcap.CurrentContext
	}{
		{name: "foreign tenant", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 2, DataScope: 2}},
		{name: "acting user", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, ActingUserID: 8}},
		{name: "impersonation", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, IsImpersonation: true}},
		{name: "acting tenant", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, ActingAsTenant: true}},
		{name: "unsupported", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, DataScopeUnsupported: true}},
		{name: "unknown scope", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 9}},
		{name: "untrusted system", current: bizctxcap.CurrentContext{TenantID: 0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := &staffDirectoryUserStub{}
			principals := &staffDirectoryPrincipalStub{}
			directory, err := NewStaffDirectory(testBizCtx(test.current), users, &currentStaffPrincipalStub{}, principals)
			if err != nil {
				t.Fatal(err)
			}
			_, err = directory.ResolveActiveBatch(context.Background(), []notificationcap.StaffDirectoryRequest{{HostUserID: 7, ExpectedVersion: 1}})
			if !bizerr.Is(err, codeCurrentStaffNotAccessible) {
				t.Fatalf("expected inaccessible result, got %v", err)
			}
			if users.calls != 0 || principals.calls != 0 {
				t.Fatalf("invalid caller reached batch readers: users=%d principals=%d", users.calls, principals.calls)
			}
		})
	}
}

func TestStaffDirectoryRejectsCustomerCallerAndInvalidBatch(t *testing.T) {
	users := &staffDirectoryUserStub{}
	principals := &staffDirectoryPrincipalStub{}
	directory, err := NewStaffDirectory(
		testBizCtx(bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}),
		users,
		&currentStaffPrincipalStub{audience: &principal.Audience{UserID: 7, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, Version: 1}},
		principals,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = directory.ResolveActiveBatch(context.Background(), nil); !bizerr.Is(err, codeCurrentStaffNotAccessible) {
		t.Fatalf("customer caller was not rejected: %v", err)
	}

	directory, err = NewStaffDirectory(
		testBizCtx(bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}),
		users,
		&currentStaffPrincipalStub{audience: &principal.Audience{UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 1}},
		principals,
	)
	if err != nil {
		t.Fatal(err)
	}
	invalid := [][]notificationcap.StaffDirectoryRequest{
		{{HostUserID: 0, ExpectedVersion: 1}},
		{{HostUserID: 7, ExpectedVersion: 0}},
		{{HostUserID: 7, ExpectedVersion: 1}, {HostUserID: 7, ExpectedVersion: 2}},
		make([]notificationcap.StaffDirectoryRequest, notificationcap.MaximumStaffDirectoryBatch+1),
	}
	for index, requests := range invalid {
		if _, resolveErr := directory.ResolveActiveBatch(context.Background(), requests); resolveErr == nil {
			t.Fatalf("invalid batch %d was accepted", index)
		}
	}
	if users.calls != 0 || principals.calls != 0 {
		t.Fatalf("invalid batches reached readers: users=%d principals=%d", users.calls, principals.calls)
	}
}

func TestStaffDirectoryAllowsExplicitBackgroundJobContext(t *testing.T) {
	users := &staffDirectoryUserStub{items: map[usercap.UserID]*usercap.UserInfo{
		"21": {ID: "21", TenantID: "0", Status: statusflag.EnabledValue},
	}}
	directory, err := NewStaffDirectory(
		testBizCtx(bizctxcap.CurrentContext{TenantID: 0, PlatformBypass: true}),
		users,
		&currentStaffPrincipalStub{err: errors.New("interactive principal must not be called")},
		&staffDirectoryPrincipalStub{states: map[int]principal.StaffDirectoryState{
			21: {UserID: 21, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 3},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := directory.ResolveActiveBatch(context.Background(), []notificationcap.StaffDirectoryRequest{{HostUserID: 21, ExpectedVersion: 3}})
	if err != nil || len(result) != 1 || !result[0].Active {
		t.Fatalf("background result=%+v err=%v", result, err)
	}
}

type testBizCtx bizctxcap.CurrentContext

func (s testBizCtx) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext(s)
}
