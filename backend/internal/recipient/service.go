package recipient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

var (
	ErrNotFound      = errors.New("recipient resource not found")
	ErrAlreadyExists = errors.New("recipient resource already exists")
	ErrInvalidInput  = errors.New("invalid recipient input")
	ErrConflict      = errors.New("recipient resource conflict")
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
	ChannelID     string
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
	ChannelID     string `json:"channel_id"`
	IdentityKind  string `json:"identity_kind"`
	IdentityValue string `json:"identity_value"`
	Verified      bool   `json:"verified"`
}

type UserProfileIdentityInput struct {
	ID            string `json:"id"`
	ProviderType  string `json:"provider_type"`
	ChannelID     string `json:"channel_id"`
	IdentityKind  string `json:"identity_kind"`
	IdentityValue string `json:"identity_value"`
	Verified      bool   `json:"verified"`
}

type UserProfileInput struct {
	User              UserInput                  `json:"user"`
	Identities        []UserProfileIdentityInput `json:"identities"`
	ExpectedUpdatedAt string                     `json:"expected_updated_at"`
}

type UserProfile struct {
	User       User
	Identities []UserIdentity
}

type UserProfileIdentityParams = UserProfileIdentityInput

type CreateUserProfileParams struct {
	User       CreateUserParams
	Identities []UserProfileIdentityParams
}

type SaveUserProfileParams struct {
	User              UpdateUserParams
	Identities        []UserProfileIdentityParams
	ExpectedUpdatedAt *time.Time
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
	CreateUserProfile(ctx context.Context, params CreateUserProfileParams) (UserProfile, error)
	GetUser(ctx context.Context, id string) (User, error)
	UpdateUser(ctx context.Context, id string, params UpdateUserParams) (User, error)
	SaveUserProfile(ctx context.Context, id string, params SaveUserProfileParams) (UserProfile, error)
	DeleteUser(ctx context.Context, id string) error

	ListUserIdentities(ctx context.Context, userID string) ([]UserIdentity, error)
	CreateUserIdentity(ctx context.Context, params CreateUserIdentityParams) (UserIdentity, error)
	UpdateUserIdentity(ctx context.Context, id string, params UpdateUserIdentityParams) (UserIdentity, error)
	DeleteUserIdentity(ctx context.Context, id string) error
	FindUserIdentity(ctx context.Context, providerType string, channelID string, identityKind string, identityValue string) (UserIdentity, error)

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

func (s *Service) CreateUserProfile(ctx context.Context, input UserProfileInput) (UserProfile, error) {
	params, err := normalizeCreateUserProfile(input)
	if err != nil {
		return UserProfile{}, err
	}
	return s.store.CreateUserProfile(ctx, params)
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

func (s *Service) SaveUserProfile(ctx context.Context, id string, input UserProfileInput) (UserProfile, error) {
	if strings.TrimSpace(id) == "" {
		return UserProfile{}, ErrInvalidInput
	}
	params, err := normalizeSaveUserProfile(input)
	if err != nil {
		return UserProfile{}, err
	}
	return s.store.SaveUserProfile(ctx, id, params)
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

func (s *Service) FindUserIdentity(ctx context.Context, providerType string, channelID string, identityKind string, identityValue string) (UserIdentity, error) {
	providerType = strings.TrimSpace(providerType)
	channelID = strings.TrimSpace(channelID)
	identityKind = strings.TrimSpace(identityKind)
	identityValue = strings.TrimSpace(identityValue)
	if providerType == "" || identityKind == "" || identityValue == "" {
		return UserIdentity{}, ErrInvalidInput
	}
	return s.store.FindUserIdentity(ctx, providerType, channelID, identityKind, identityValue)
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
	if !isNumericOrgCode(input.Code) {
		return CreateOrgUnitParams{}, ErrInvalidInput
	}
	if input.ParentID == "" {
		if len(input.Code) != 4 {
			return CreateOrgUnitParams{}, ErrInvalidInput
		}
	} else if len(input.Code) < 6 || len(input.Code)%2 != 0 {
		return CreateOrgUnitParams{}, ErrInvalidInput
	}
	return input, nil
}

func isNumericOrgCode(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
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
	if err := validateUserFallbackAttributes(attrs); err != nil {
		return CreateUserParams{}, err
	}
	input.Attributes = attrs
	return input, nil
}

func normalizeUserIdentity(input UserIdentityInput) (CreateUserIdentityParams, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ProviderType = strings.TrimSpace(input.ProviderType)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.IdentityKind = strings.TrimSpace(input.IdentityKind)
	input.IdentityValue = strings.TrimSpace(input.IdentityValue)
	if input.UserID == "" || input.ProviderType == "" || input.IdentityKind == "" || input.IdentityValue == "" {
		return CreateUserIdentityParams{}, ErrInvalidInput
	}
	if !validIdentityValue(input.IdentityKind, input.IdentityValue) {
		return CreateUserIdentityParams{}, ErrInvalidInput
	}
	return input, nil
}

func normalizeCreateUserProfile(input UserProfileInput) (CreateUserProfileParams, error) {
	user, err := normalizeUser(input.User)
	if err != nil {
		return CreateUserProfileParams{}, err
	}
	identities, err := normalizeUserProfileIdentities(input.Identities)
	if err != nil {
		return CreateUserProfileParams{}, err
	}
	for _, identity := range identities {
		if identity.ID != "" {
			return CreateUserProfileParams{}, ErrInvalidInput
		}
	}
	return CreateUserProfileParams{User: user, Identities: identities}, nil
}

func normalizeSaveUserProfile(input UserProfileInput) (SaveUserProfileParams, error) {
	user, err := normalizeUser(input.User)
	if err != nil {
		return SaveUserProfileParams{}, err
	}
	identities, err := normalizeUserProfileIdentities(input.Identities)
	if err != nil {
		return SaveUserProfileParams{}, err
	}
	var expectedUpdatedAt *time.Time
	if strings.TrimSpace(input.ExpectedUpdatedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(input.ExpectedUpdatedAt))
		if err != nil {
			return SaveUserProfileParams{}, ErrInvalidInput
		}
		expectedUpdatedAt = &parsed
	}
	return SaveUserProfileParams{User: user, Identities: identities, ExpectedUpdatedAt: expectedUpdatedAt}, nil
}

func normalizeUserProfileIdentities(inputs []UserProfileIdentityInput) ([]UserProfileIdentityParams, error) {
	identities := make([]UserProfileIdentityParams, 0, len(inputs))
	seenIDs := map[string]bool{}
	for _, input := range inputs {
		input.ID = strings.TrimSpace(input.ID)
		input.ProviderType = strings.TrimSpace(input.ProviderType)
		input.ChannelID = strings.TrimSpace(input.ChannelID)
		input.IdentityKind = strings.TrimSpace(input.IdentityKind)
		input.IdentityValue = strings.TrimSpace(input.IdentityValue)
		if input.ProviderType == "" || input.IdentityKind == "" || input.IdentityValue == "" {
			return nil, ErrInvalidInput
		}
		if input.ID != "" {
			if seenIDs[input.ID] {
				return nil, ErrInvalidInput
			}
			seenIDs[input.ID] = true
		}
		if !validIdentityValue(input.IdentityKind, input.IdentityValue) {
			return nil, ErrInvalidInput
		}
		identities = append(identities, input)
	}
	return identities, nil
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

func validIdentityValue(kind string, value string) bool {
	switch kind {
	case "email":
		address, err := mail.ParseAddress(value)
		return err == nil && address.Address == value && strings.Contains(value, "@")
	case "mobile":
		normalized := strings.TrimPrefix(value, "+")
		if len(normalized) < 6 || len(normalized) > 20 {
			return false
		}
		for _, char := range normalized {
			if !unicode.IsDigit(char) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func validateUserFallbackAttributes(raw json.RawMessage) error {
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(raw, &attributes); err != nil {
		return ErrInvalidInput
	}
	for _, kind := range []string{"email", "mobile"} {
		value, ok := attributes[kind]
		if !ok || string(bytes.TrimSpace(value)) == "null" {
			continue
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return ErrInvalidInput
		}
		text = strings.TrimSpace(text)
		if text != "" && !validIdentityValue(kind, text) {
			return ErrInvalidInput
		}
	}
	return nil
}
