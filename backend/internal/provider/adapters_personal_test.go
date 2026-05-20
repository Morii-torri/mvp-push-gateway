package provider

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestBuildDeliveryRequestServerChanRejectsMalformedRecipientSendKey(t *testing.T) {
	_, err := BuildDeliveryRequest(Channel{
		ProviderType: ProviderServerChan,
		SendConfig:   json.RawMessage(`{"url":"https://<uid>.push.ft07.com/send/<sendkey>.send"}`),
	}, BuildDeliveryRequestInput{
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderServerChan,
			MessageType:  "markdown",
			Content:      json.RawMessage(`{"title":"ServerChan title"}`),
		},
		ResolvedRecipients: []ResolvedRecipient{
			{PlatformIDs: map[string]string{"serverchan_sendkey": "bad-key"}},
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for malformed ServerChan sendKey, got %v", err)
	}
}
