package recipient

import "testing"

func TestNormalizeOrgUnitRequiresHierarchicalNumericCodeShape(t *testing.T) {
	cases := []struct {
		name    string
		input   OrgUnitInput
		wantErr bool
	}{
		{
			name: "root code uses four digits",
			input: OrgUnitInput{
				Code: "1000",
				Name: "根组织",
			},
		},
		{
			name: "root code rejects six digits",
			input: OrgUnitInput{
				Code: "100001",
				Name: "根组织",
			},
			wantErr: true,
		},
		{
			name: "child code uses at least six even digits",
			input: OrgUnitInput{
				ParentID: "parent-1",
				Code:     "100001",
				Name:     "子组织",
			},
		},
		{
			name: "child code rejects root length",
			input: OrgUnitInput{
				ParentID: "parent-1",
				Code:     "1000",
				Name:     "子组织",
			},
			wantErr: true,
		},
		{
			name: "code rejects non digits",
			input: OrgUnitInput{
				Code: "ops1",
				Name: "根组织",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeOrgUnit(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected invalid org unit input to fail")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected org unit input to pass, got %v", err)
			}
		})
	}
}

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

func TestNormalizeUserIdentityRejectsInvalidEmailAndMobile(t *testing.T) {
	cases := []UserIdentityInput{
		{
			UserID:        "user-1",
			ProviderType:  "email",
			IdentityKind:  "email",
			IdentityValue: "not-an-email",
		},
		{
			UserID:        "user-1",
			ProviderType:  "aliyun_sms",
			IdentityKind:  "mobile",
			IdentityValue: "abc",
		},
	}

	for _, input := range cases {
		if _, err := normalizeUserIdentity(input); err == nil {
			t.Fatalf("expected invalid identity value %q for kind %q to fail", input.IdentityValue, input.IdentityKind)
		}
	}
}

func TestNormalizeUserRejectsInvalidFallbackEmailAndMobile(t *testing.T) {
	cases := []UserInput{
		{
			DisplayName: "Invalid Email",
			Enabled:     true,
			Attributes:  []byte(`{"email":"not-an-email"}`),
		},
		{
			DisplayName: "Invalid Mobile",
			Enabled:     true,
			Attributes:  []byte(`{"mobile":"abc"}`),
		},
	}

	for _, input := range cases {
		if _, err := normalizeUser(input); err == nil {
			t.Fatalf("expected invalid fallback attributes to fail: %s", input.Attributes)
		}
	}
}

func TestNormalizeCreateUserProfileRejectsExistingIdentityID(t *testing.T) {
	_, err := normalizeCreateUserProfile(UserProfileInput{
		User: UserInput{
			DisplayName: "张三",
			Enabled:     true,
			Attributes:  []byte(`{}`),
		},
		Identities: []UserProfileIdentityInput{
			{
				ID:            "identity-1",
				ProviderType:  "email",
				IdentityKind:  "email",
				IdentityValue: "zhangsan@example.com",
				Verified:      true,
			},
		},
	})
	if err == nil {
		t.Fatal("expected create user profile with existing identity id to fail")
	}
}
