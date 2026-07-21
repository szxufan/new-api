package types

type ChannelError struct {
	ChannelId     int    `json:"channel_id"`
	ChannelType   int    `json:"channel_type"`
	ChannelName   string `json:"channel_name"`
	IsMultiKey    bool   `json:"is_multi_key"`
	AutoBan       bool   `json:"auto_ban"`
	UsingKey      string `json:"using_key"`
	Disable429Ban bool   `json:"disable_429_ban"` // 是否禁用429自动限流；true=跳过限流走正常重试
}

func NewChannelError(channelId int, channelType int, channelName string, isMultiKey bool, usingKey string, autoBan bool, disable429Ban bool) *ChannelError {
	return &ChannelError{
		ChannelId:     channelId,
		ChannelType:   channelType,
		ChannelName:   channelName,
		IsMultiKey:    isMultiKey,
		AutoBan:       autoBan,
		UsingKey:      usingKey,
		Disable429Ban: disable429Ban,
	}
}
