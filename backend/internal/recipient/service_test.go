package recipient

import "testing"

func TestNormalizeUserIdentityKeepsOptionalChannelScope(t *testing.T) {
	input := UserIdentityInput{
		UserID:        " user-1 ",
		ProviderType:  " email ",
		ChannelID:     " channel-email-work ",
		IdentityKind:  " email ",
		IdentityValue: " work@example.com ",
		Verified:      true,
	}

	normalized, err := normalizeUserIdentity(input)
	if err != nil {
		t.Fatalf("normalize user identity: %v", err)
	}
	if normalized.UserID != "user-1" ||
		normalized.ProviderType != "email" ||
		normalized.ChannelID != "channel-email-work" ||
		normalized.IdentityKind != "email" ||
		normalized.IdentityValue != "work@example.com" {
		t.Fatalf("unexpected normalized identity: %+v", normalized)
	}
}
