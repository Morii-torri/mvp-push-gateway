package recipient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound      = errors.New("recipient resource not found")
	ErrAlreadyExists = errors.New("recipient resource already exists")
	ErrInvalidInput  = errors.New("invalid recipient input")
)

type OrgUnit struct {
	ID        string
	ParentID  string
	Code      string
	Name      string
	SortOrder int
	Path      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID           string
	DisplayName  string
	PrimaryOrgID string
	Enabled      bool
	Attributes   json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserIdentity struct {
	ID            string
	UserID        string
	ProviderType  string
	IdentityKind  string
	IdentityValue string
	Verified      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RecipientGroup struct {
	ID              string
	Name            string
	UserIDs         []string
	OrgIDs          []string
	ExcludedUserIDs []string
	ExcludedOrgIDs  []string
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type OrgUnitInput struct {
	ParentID  string `json:"parent_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type UserInput struct {
	DisplayName  string          `json:"display_name"`
	PrimaryOrgID string          `json:"primary_org_id"`
	Enabled      bool            `json:"enabled"`
	Attributes   json.RawMessage `json:"attributes"`
}

type UserIdentityInput struct {
	UserID        string `json:"user_id"`
	ProviderType  string `json:"provider_type"`
	IdentityKind  string `json:"identity_kind"`
	IdentityValue string `json:"identity_value"`
	Verified      bool   `json:"verified"`
}

type RecipientGroupInput struct {
	Name            string   `json:"name"`
	UserIDs         []string `json:"user_ids"`
	OrgIDs          []string `json:"org_ids"`
	ExcludedUserIDs []string `json:"excluded_user_ids"`
	ExcludedOrgIDs  []string `json:"excluded_org_ids"`
	Enabled         bool     `json:"enabled"`
}

type CreateOrgUnitParams = OrgUnitInput
type UpdateOrgUnitParams = OrgUnitInput
type CreateUserParams = UserInput
type UpdateUserParams = UserInput
type CreateUserIdentityParams = UserIdentityInput
type UpdateUserIdentityParams = UserIdentityInput
type CreateRecipientGroupParams = RecipientGroupInput
type UpdateRecipientGroupParams = RecipientGroupInput

type Store interface {
	ListOrgUnits(ctx context.Context) ([]OrgUnit, error)
	CreateOrgUnit(ctx context.Context, params CreateOrgUnitParams) (OrgUnit, error)
	GetOrgUnit(ctx context.Context, id string) (OrgUnit, error)
	UpdateOrgUnit(ctx context.Context, id string, params UpdateOrgUnitParams) (OrgUnit, error)
	DeleteOrgUnit(ctx context.Context, id string) error

	ListUsers(ctx context.Context) ([]User, error)
	CreateUser(ctx context.Context, params CreateUserParams) (User, error)
	GetUser(ctx context.Context, id string) (User, error)
	UpdateUser(ctx context.Context, id string, params UpdateUserParams) (User, error)
	DeleteUser(ctx context.Context, id string) error

	ListUserIdentities(ctx context.Context, userID string) ([]UserIdentity, error)
	CreateUserIdentity(ctx context.Context, params CreateUserIdentityParams) (UserIdentity, error)
	UpdateUserIdentity(ctx context.Context, id string, params UpdateUserIdentityParams) (UserIdentity, error)
	DeleteUserIdentity(ctx context.Context, id string) error
	FindUserIdentity(ctx context.Context, providerType string, identityKind string, identityValue string) (UserIdentity, error)

	ListRecipientGroups(ctx context.Context) ([]RecipientGroup, error)
	CreateRecipientGroup(ctx context.Context, params CreateRecipientGroupParams) (RecipientGroup, error)
	GetRecipientGroup(ctx context.Context, id string) (RecipientGroup, error)
	UpdateRecipientGroup(ctx context.Context, id string, params UpdateRecipientGroupParams) (RecipientGroup, error)
	DeleteRecipientGroup(ctx context.Context, id string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListOrgUnits(ctx context.Context) ([]OrgUnit, error) {
	return s.store.ListOrgUnits(ctx)
}

func (s *Service) CreateOrgUnit(ctx context.Context, input OrgUnitInput) (OrgUnit, error) {
	params, err := normalizeOrgUnit(input)
	if err != nil {
		return OrgUnit{}, err
	}
	return s.store.CreateOrgUnit(ctx, params)
}

func (s *Service) GetOrgUnit(ctx context.Context, id string) (OrgUnit, error) {
	if strings.TrimSpace(id) == "" {
		return OrgUnit{}, ErrInvalidInput
	}
	return s.store.GetOrgUnit(ctx, id)
}

func (s *Service) UpdateOrgUnit(ctx context.Context, id string, input OrgUnitInput) (OrgUnit, error) {
	if strings.TrimSpace(id) == "" {
		return OrgUnit{}, ErrInvalidInput
	}
	params, err := normalizeOrgUnit(input)
	if err != nil {
		return OrgUnit{}, err
	}
	return s.store.UpdateOrgUnit(ctx, id, params)
}

func (s *Service) DeleteOrgUnit(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteOrgUnit(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.store.ListUsers(ctx)
}

func (s *Service) CreateUser(ctx context.Context, input UserInput) (User, error) {
	params, err := normalizeUser(input)
	if err != nil {
		return User{}, err
	}
	return s.store.CreateUser(ctx, params)
}

func (s *Service) GetUser(ctx context.Context, id string) (User, error) {
	if strings.TrimSpace(id) == "" {
		return User{}, ErrInvalidInput
	}
	return s.store.GetUser(ctx, id)
}

func (s *Service) UpdateUser(ctx context.Context, id string, input UserInput) (User, error) {
	if strings.TrimSpace(id) == "" {
		return User{}, ErrInvalidInput
	}
	params, err := normalizeUser(input)
	if err != nil {
		return User{}, err
	}
	return s.store.UpdateUser(ctx, id, params)
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteUser(ctx, id)
}

func (s *Service) ListUserIdentities(ctx context.Context, userID string) ([]UserIdentity, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidInput
	}
	return s.store.ListUserIdentities(ctx, userID)
}

func (s *Service) CreateUserIdentity(ctx context.Context, input UserIdentityInput) (UserIdentity, error) {
	params, err := normalizeUserIdentity(input)
	if err != nil {
		return UserIdentity{}, err
	}
	return s.store.CreateUserIdentity(ctx, params)
}

func (s *Service) UpdateUserIdentity(ctx context.Context, id string, input UserIdentityInput) (UserIdentity, error) {
	if strings.TrimSpace(id) == "" {
		return UserIdentity{}, ErrInvalidInput
	}
	params, err := normalizeUserIdentity(input)
	if err != nil {
		return UserIdentity{}, err
	}
	return s.store.UpdateUserIdentity(ctx, id, params)
}

func (s *Service) DeleteUserIdentity(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteUserIdentity(ctx, id)
}

func (s *Service) FindUserIdentity(ctx context.Context, providerType string, identityKind string, identityValue string) (UserIdentity, error) {
	providerType = strings.TrimSpace(providerType)
	identityKind = strings.TrimSpace(identityKind)
	identityValue = strings.TrimSpace(identityValue)
	if providerType == "" || identityKind == "" || identityValue == "" {
		return UserIdentity{}, ErrInvalidInput
	}
	return s.store.FindUserIdentity(ctx, providerType, identityKind, identityValue)
}

func (s *Service) ListRecipientGroups(ctx context.Context) ([]RecipientGroup, error) {
	return s.store.ListRecipientGroups(ctx)
}

func (s *Service) CreateRecipientGroup(ctx context.Context, input RecipientGroupInput) (RecipientGroup, error) {
	params, err := normalizeRecipientGroup(input)
	if err != nil {
		return RecipientGroup{}, err
	}
	return s.store.CreateRecipientGroup(ctx, params)
}

func (s *Service) GetRecipientGroup(ctx context.Context, id string) (RecipientGroup, error) {
	if strings.TrimSpace(id) == "" {
		return RecipientGroup{}, ErrInvalidInput
	}
	return s.store.GetRecipientGroup(ctx, id)
}

func (s *Service) UpdateRecipientGroup(ctx context.Context, id string, input RecipientGroupInput) (RecipientGroup, error) {
	if strings.TrimSpace(id) == "" {
		return RecipientGroup{}, ErrInvalidInput
	}
	params, err := normalizeRecipientGroup(input)
	if err != nil {
		return RecipientGroup{}, err
	}
	return s.store.UpdateRecipientGroup(ctx, id, params)
}

func (s *Service) DeleteRecipientGroup(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteRecipientGroup(ctx, id)
}

func normalizeOrgUnit(input OrgUnitInput) (CreateOrgUnitParams, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.ParentID = strings.TrimSpace(input.ParentID)
	if input.Code == "" || input.Name == "" {
		return CreateOrgUnitParams{}, ErrInvalidInput
	}
	return input, nil
}

func normalizeUser(input UserInput) (CreateUserParams, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.PrimaryOrgID = strings.TrimSpace(input.PrimaryOrgID)
	if input.DisplayName == "" {
		return CreateUserParams{}, ErrInvalidInput
	}
	attrs, err := normalizeJSON(input.Attributes)
	if err != nil {
		return CreateUserParams{}, err
	}
	input.Attributes = attrs
	return input, nil
}

func normalizeUserIdentity(input UserIdentityInput) (CreateUserIdentityParams, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ProviderType = strings.TrimSpace(input.ProviderType)
	input.IdentityKind = strings.TrimSpace(input.IdentityKind)
	input.IdentityValue = strings.TrimSpace(input.IdentityValue)
	if input.UserID == "" || input.ProviderType == "" || input.IdentityKind == "" || input.IdentityValue == "" {
		return CreateUserIdentityParams{}, ErrInvalidInput
	}
	return input, nil
}

func normalizeRecipientGroup(input RecipientGroupInput) (CreateRecipientGroupParams, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return CreateRecipientGroupParams{}, ErrInvalidInput
	}
	input.UserIDs = clean(input.UserIDs)
	input.OrgIDs = clean(input.OrgIDs)
	input.ExcludedUserIDs = clean(input.ExcludedUserIDs)
	input.ExcludedOrgIDs = clean(input.ExcludedOrgIDs)
	return input, nil
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

func clean(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}
