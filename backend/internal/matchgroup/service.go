package matchgroup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"time"
)

var (
	ErrNotFound      = errors.New("match group not found")
	ErrAlreadyExists = errors.New("match group already exists")
	ErrInvalidInput  = errors.New("invalid match group input")
	ErrInUse         = errors.New("match group is referenced")
)

type Group struct {
	ID             string
	Name           string
	GroupType      string
	Description    string
	Enabled        bool
	ItemCount      int
	ReferenceCount int
	Items          []Item
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Item struct {
	ID        string
	GroupID   string
	Value     string
	ValueType string
	Metadata  json.RawMessage
	CreatedAt time.Time
}

type GroupInput struct {
	Name        string `json:"name"`
	GroupType   string `json:"group_type"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type ItemInput struct {
	Value     string          `json:"value"`
	ValueType string          `json:"value_type"`
	Metadata  json.RawMessage `json:"metadata"`
}

type CreateGroupParams = GroupInput
type UpdateGroupParams = GroupInput
type CreateItemParams = ItemInput
type UpdateItemParams = ItemInput

type Store interface {
	ListGroups(ctx context.Context) ([]Group, error)
	CreateGroup(ctx context.Context, params CreateGroupParams) (Group, error)
	GetGroup(ctx context.Context, id string) (Group, error)
	UpdateGroup(ctx context.Context, id string, params UpdateGroupParams) (Group, error)
	GroupReferenced(ctx context.Context, id string) (bool, error)
	DeleteGroup(ctx context.Context, id string) error
	ListItems(ctx context.Context, groupID string) ([]Item, error)
	CreateItem(ctx context.Context, groupID string, params CreateItemParams) (Item, error)
	GetItem(ctx context.Context, groupID string, itemID string) (Item, error)
	UpdateItem(ctx context.Context, groupID string, itemID string, params UpdateItemParams) (Item, error)
	DeleteItem(ctx context.Context, groupID string, itemID string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListGroups(ctx context.Context) ([]Group, error) {
	return s.store.ListGroups(ctx)
}

func (s *Service) CreateGroup(ctx context.Context, input GroupInput) (Group, error) {
	params, err := normalizeGroup(input)
	if err != nil {
		return Group{}, err
	}
	return s.store.CreateGroup(ctx, params)
}

func (s *Service) GetGroup(ctx context.Context, id string) (Group, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Group{}, ErrInvalidInput
	}
	return s.store.GetGroup(ctx, id)
}

func (s *Service) UpdateGroup(ctx context.Context, id string, input GroupInput) (Group, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Group{}, ErrInvalidInput
	}
	params, err := normalizeGroup(input)
	if err != nil {
		return Group{}, err
	}
	return s.store.UpdateGroup(ctx, id, params)
}

func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidInput
	}
	referenced, err := s.store.GroupReferenced(ctx, id)
	if err != nil {
		return err
	}
	if referenced {
		return ErrInUse
	}
	return s.store.DeleteGroup(ctx, id)
}

func (s *Service) ListItems(ctx context.Context, groupID string) ([]Item, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, ErrInvalidInput
	}
	return s.store.ListItems(ctx, groupID)
}

func (s *Service) CreateItem(ctx context.Context, groupID string, input ItemInput) (Item, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return Item{}, ErrInvalidInput
	}
	group, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return Item{}, err
	}
	params, err := normalizeItem(input, group.GroupType)
	if err != nil {
		return Item{}, err
	}
	return s.store.CreateItem(ctx, groupID, params)
}

func (s *Service) GetItem(ctx context.Context, groupID string, itemID string) (Item, error) {
	groupID = strings.TrimSpace(groupID)
	itemID = strings.TrimSpace(itemID)
	if groupID == "" || itemID == "" {
		return Item{}, ErrInvalidInput
	}
	return s.store.GetItem(ctx, groupID, itemID)
}

func (s *Service) UpdateItem(ctx context.Context, groupID string, itemID string, input ItemInput) (Item, error) {
	groupID = strings.TrimSpace(groupID)
	itemID = strings.TrimSpace(itemID)
	if groupID == "" || itemID == "" {
		return Item{}, ErrInvalidInput
	}
	group, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return Item{}, err
	}
	params, err := normalizeItem(input, group.GroupType)
	if err != nil {
		return Item{}, err
	}
	return s.store.UpdateItem(ctx, groupID, itemID, params)
}

func (s *Service) DeleteItem(ctx context.Context, groupID string, itemID string) error {
	groupID = strings.TrimSpace(groupID)
	itemID = strings.TrimSpace(itemID)
	if groupID == "" || itemID == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteItem(ctx, groupID, itemID)
}

func normalizeGroup(input GroupInput) (CreateGroupParams, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.GroupType = strings.TrimSpace(input.GroupType)
	input.Description = strings.TrimSpace(input.Description)
	if input.GroupType == "" {
		input.GroupType = "text"
	}
	if input.Name == "" || !validGroupType(input.GroupType) {
		return CreateGroupParams{}, ErrInvalidInput
	}
	input.Enabled = true
	return input, nil
}

func validGroupType(groupType string) bool {
	switch groupType {
	case "text", "ip":
		return true
	default:
		return false
	}
}

func normalizeItem(input ItemInput, groupType string) (CreateItemParams, error) {
	input.Value = strings.TrimSpace(input.Value)
	input.ValueType = strings.TrimSpace(input.ValueType)
	if input.Value == "" {
		return CreateItemParams{}, ErrInvalidInput
	}
	groupType = strings.TrimSpace(groupType)
	if groupType == "ip" {
		valueType, err := normalizeIPItemValueType(input.Value)
		if err != nil {
			return CreateItemParams{}, err
		}
		input.ValueType = valueType
	} else if input.ValueType == "" {
		input.ValueType = "text"
	}
	metadata, err := normalizeJSON(input.Metadata)
	if err != nil {
		return CreateItemParams{}, err
	}
	input.Metadata = metadata
	return input, nil
}

func normalizeIPItemValueType(value string) (string, error) {
	if _, err := netip.ParseAddr(value); err == nil {
		return "ip", nil
	}
	if _, err := netip.ParsePrefix(value); err == nil {
		return "cidr", nil
	}
	return "", ErrInvalidInput
}

func normalizeJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(raw) {
		return nil, ErrInvalidInput
	}
	return append(json.RawMessage(nil), bytes.TrimSpace(raw)...), nil
}
