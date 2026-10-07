package v1

type ChannelSecretStatus struct {
	ChannelNo   string `json:"channelNo" dc:"Opaque Provider channel number" eg:"channel_a1"`
	BindingName string `json:"bindingName" dc:"Adapter credential slot name" eg:"password"`
	Configured  bool   `json:"configured" dc:"Whether an active encrypted secret is configured" eg:"true"`
	MaskedTail  string `json:"maskedTail" dc:"Bounded display-only masked suffix" eg:"1234"`
	UpdatedAt   int64  `json:"updatedAt" dc:"Secret lifecycle fence in Unix milliseconds" eg:"1767225600000"`
}

type ChannelSecretAccessContext struct {
	Token     string `json:"token" dc:"Opaque signed single-use access context" eg:"ra1.key_01.opaque.signature"`
	ExpiresAt int64  `json:"expiresAt" dc:"Access-context expiry in Unix milliseconds" eg:"1767225900000"`
}
