package provider

func builtInRequestConfig(channel Channel, input BuildRequestInput) (requestConfig, bool, error) {
	auth, err := decodeObjectConfig(channel.AuthConfig)
	if err != nil {
		return requestConfig{}, false, err
	}
	send, err := decodeObjectConfig(channel.SendConfig)
	if err != nil {
		return requestConfig{}, false, err
	}
	content, err := decodeObjectConfig(input.Body)
	if err != nil {
		return requestConfig{}, false, err
	}

	switch channel.ProviderType {
	case ProviderWebhook:
		config, err := webhookRequestConfig(send, content, input.Recipient)
		return config, true, err
	case ProviderPushPlus:
		config, err := pushPlusRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderWxPusher:
		config, err := wxPusherRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderServerChan:
		config, err := serverChanRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderNtfy:
		config, err := ntfyRequestConfig(auth, send, content)
		return config, true, err
	case ProviderGotify:
		config, err := gotifyRequestConfig(auth, send, content)
		return config, true, err
	case ProviderBark:
		config, err := barkRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderPushMe:
		config, err := pushMeRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderSelf:
		config, err := selfRequestConfig(auth, send, content, input)
		return config, true, err
	case ProviderEmail:
		config, err := emailRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderAliyunSMS, ProviderTencentSMS, ProviderBaiduSMS:
		config, err := smsRequestConfig(channel.ProviderType, auth, send, content, input.Recipient)
		return config, true, err
	case ProviderWeComRobot:
		config, err := weComRobotRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderWeComApp:
		config, err := weComAppRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderDingTalkRobot:
		config, err := dingTalkRobotRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderDingTalkWork:
		config, err := dingTalkWorkRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderFeishuRobot:
		config, err := feishuRobotRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderFeishuGroup:
		config, err := feishuGroupRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	default:
		return requestConfig{}, false, nil
	}
}
