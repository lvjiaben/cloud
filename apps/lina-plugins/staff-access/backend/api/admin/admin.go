package admin

import (
	"context"
	v1 "lina-plugin-staff-access/backend/api/admin/v1"
)

type IAdminV1 interface {
	GrantList(context.Context, *v1.GrantListReq) (*v1.GrantListRes, error)
	GrantSave(context.Context, *v1.GrantSaveReq) (*v1.GrantSaveRes, error)
	UserCandidateList(context.Context, *v1.UserCandidateListReq) (*v1.UserCandidateListRes, error)
}
