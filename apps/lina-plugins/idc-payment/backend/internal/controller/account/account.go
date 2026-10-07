package account

import "lina-plugin-idc-payment/backend/api/account"

var _ account.IAccountV1 = (*ControllerV1)(nil)
