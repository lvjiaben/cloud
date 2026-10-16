// This file verifies organization disablement, bounded candidate contracts and
// explicit constructor dependencies without reading organization private tables.

package supportdesk

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/orgcap"
)

type candidateOrganizationStub struct {
	orgcap.Service
	enabled    bool
	department *candidateDepartmentStub
}

func (s candidateOrganizationStub) Available(context.Context) bool       { return s.enabled }
func (s candidateOrganizationStub) Department() orgcap.DepartmentService { return s.department }
func (s candidateOrganizationStub) Assignment() orgcap.AssignmentService {
	return queueAssignmentStub{}
}

type candidateDepartmentStub struct {
	orgcap.DepartmentService
	received orgcap.DeptListInput
	fail     error
}

func (s *candidateDepartmentStub) List(_ context.Context, input orgcap.DeptListInput) (*capmodel.PageResult[*orgcap.DeptInfo], error) {
	s.received = input
	if s.fail != nil {
		return nil, s.fail
	}
	return &capmodel.PageResult[*orgcap.DeptInfo]{Items: []*orgcap.DeptInfo{{DeptID: 31, DeptName: "Support", Status: 1}}, Total: 47}, nil
}

func TestOrganizationCandidatesAreBoundedAndFailClosed(t *testing.T) {
	ctx := context.Background()
	dept := &candidateDepartmentStub{}
	svc := &serviceImpl{bizCtx: queueBizCtxStub{scope: 1}, principal: supportDeskPrincipalStub{}, authorization: queueAuthStub{}, organization: candidateOrganizationStub{enabled: true, department: dept}}
	result, err := svc.OrganizationDepartments(ctx, " support ", 2, 20)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 47 || len(result.Items) != 1 || dept.received.Page.PageNum != 2 || dept.received.Page.PageSize != 20 || dept.received.Keyword != "support" || dept.received.Status == nil || *dept.received.Status != 1 {
		t.Fatalf("unexpected bounded candidate result: %+v input=%+v", result, dept.received)
	}
	if _, err = svc.OrganizationDepartments(ctx, "", 1, 101); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("oversize=%v", err)
	}
	svc.organization = candidateOrganizationStub{enabled: false}
	result, err = svc.OrganizationDepartments(ctx, "", 1, 20)
	if err != nil || result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("disabled candidates=%+v %v", result, err)
	}
	if err = svc.requireOrganizationBinding(ctx, 31); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("disabled binding=%v", err)
	}
	svc.bizCtx = queueBizCtxStub{scope: 3}
	if _, err = svc.OrganizationDepartments(ctx, "", 1, 20); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("department scope global settings=%v", err)
	}
}
