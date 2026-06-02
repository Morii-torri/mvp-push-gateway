package matchgroup

import "testing"

func TestNormalizeGroupAllowsOnlyTextAndIPTypes(t *testing.T) {
	defaulted, err := normalizeGroup(GroupInput{Name: "默认文本组"})
	if err != nil {
		t.Fatalf("normalize default group: %v", err)
	}
	if defaulted.GroupType != "text" {
		t.Fatalf("expected empty group type to default to text, got %q", defaulted.GroupType)
	}
	if !defaulted.Enabled {
		t.Fatal("expected match groups to be normalized as enabled")
	}

	disabledInput, err := normalizeGroup(GroupInput{Name: "隐藏状态", GroupType: "text", Enabled: false})
	if err != nil {
		t.Fatalf("normalize disabled input: %v", err)
	}
	if !disabledInput.Enabled {
		t.Fatal("expected disabled input to be normalized as enabled")
	}

	for _, groupType := range []string{"text", "ip"} {
		if _, err := normalizeGroup(GroupInput{Name: "有效组", GroupType: groupType}); err != nil {
			t.Fatalf("expected %q to be valid: %v", groupType, err)
		}
	}

	for _, groupType := range []string{"business", "system"} {
		if _, err := normalizeGroup(GroupInput{Name: "旧类型", GroupType: groupType}); err != ErrInvalidInput {
			t.Fatalf("expected %q to be rejected, got %v", groupType, err)
		}
	}
}
