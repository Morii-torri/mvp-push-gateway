package matchgroup

import (
	"context"
	"errors"
	"testing"
)

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

func TestCreateItemValidatesIPGroupValues(t *testing.T) {
	store := newMemoryMatchGroupStore()
	service := NewService(store)
	ctx := context.Background()

	if _, err := service.CreateItem(ctx, "ip-group", ItemInput{Value: "not-an-ip"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid IP group value to be rejected, got %v", err)
	}
	singleIP, err := service.CreateItem(ctx, "ip-group", ItemInput{Value: "10.1.2.3"})
	if err != nil {
		t.Fatalf("expected single IP to be accepted: %v", err)
	}
	if singleIP.ValueType != "ip" {
		t.Fatalf("expected single IP value type to be normalized, got %q", singleIP.ValueType)
	}
	prefix, err := service.CreateItem(ctx, "ip-group", ItemInput{Value: "10.1.0.0/16"})
	if err != nil {
		t.Fatalf("expected CIDR prefix to be accepted: %v", err)
	}
	if prefix.ValueType != "cidr" {
		t.Fatalf("expected CIDR value type to be normalized, got %q", prefix.ValueType)
	}
}

func TestDeleteGroupRejectsReferencedMatchGroup(t *testing.T) {
	store := newMemoryMatchGroupStore()
	store.referencedGroups["ip-group"] = true
	service := NewService(store)

	err := service.DeleteGroup(context.Background(), "ip-group")
	if !errors.Is(err, ErrInUse) {
		t.Fatalf("expected referenced match group delete to be rejected, got %v", err)
	}
}

type memoryMatchGroupStore struct {
	groups           map[string]Group
	items            []Item
	referencedGroups map[string]bool
}

func newMemoryMatchGroupStore() *memoryMatchGroupStore {
	return &memoryMatchGroupStore{
		groups: map[string]Group{
			"ip-group":   {ID: "ip-group", Name: "IP 组", GroupType: "ip", Enabled: true},
			"text-group": {ID: "text-group", Name: "文本组", GroupType: "text", Enabled: true},
		},
		referencedGroups: map[string]bool{},
	}
}

func (s *memoryMatchGroupStore) ListGroups(context.Context) ([]Group, error) {
	items := make([]Group, 0, len(s.groups))
	for _, group := range s.groups {
		items = append(items, group)
	}
	return items, nil
}

func (s *memoryMatchGroupStore) CreateGroup(_ context.Context, params CreateGroupParams) (Group, error) {
	group := Group{ID: "created-group", Name: params.Name, GroupType: params.GroupType, Enabled: params.Enabled}
	s.groups[group.ID] = group
	return group, nil
}

func (s *memoryMatchGroupStore) GetGroup(_ context.Context, id string) (Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return Group{}, ErrNotFound
	}
	return group, nil
}

func (s *memoryMatchGroupStore) UpdateGroup(_ context.Context, id string, params UpdateGroupParams) (Group, error) {
	group := Group{ID: id, Name: params.Name, GroupType: params.GroupType, Enabled: params.Enabled}
	s.groups[id] = group
	return group, nil
}

func (s *memoryMatchGroupStore) GroupReferenced(_ context.Context, id string) (bool, error) {
	return s.referencedGroups[id], nil
}

func (s *memoryMatchGroupStore) DeleteGroup(context.Context, string) error {
	return nil
}

func (s *memoryMatchGroupStore) ListItems(context.Context, string) ([]Item, error) {
	return append([]Item(nil), s.items...), nil
}

func (s *memoryMatchGroupStore) CreateItem(_ context.Context, groupID string, params CreateItemParams) (Item, error) {
	item := Item{ID: "item-" + params.Value, GroupID: groupID, Value: params.Value, ValueType: params.ValueType, Metadata: params.Metadata}
	s.items = append(s.items, item)
	return item, nil
}

func (s *memoryMatchGroupStore) GetItem(context.Context, string, string) (Item, error) {
	return Item{}, ErrNotFound
}

func (s *memoryMatchGroupStore) UpdateItem(_ context.Context, groupID string, itemID string, params UpdateItemParams) (Item, error) {
	return Item{ID: itemID, GroupID: groupID, Value: params.Value, ValueType: params.ValueType, Metadata: params.Metadata}, nil
}

func (s *memoryMatchGroupStore) DeleteItem(context.Context, string, string) error {
	return nil
}
