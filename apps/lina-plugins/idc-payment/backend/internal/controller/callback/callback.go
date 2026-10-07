package callback

import callbackapi "lina-plugin-idc-payment/backend/api/callback"

var _ callbackapi.ICallbackV1 = (*ControllerV1)(nil)
