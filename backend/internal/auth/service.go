package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrSetupClosed        = errors.New("setup is closed")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
)

const defaultSessionTTL = 24 * time.Hour

type Store interface {
	GetSetupStatus(ctx context.Context) (SetupStatus, error)
	CreateFirstAdmin(ctx context.Context, params CreateFirstAdminParams) (Admin, error)
	FindAdminByUsername(ctx context.Context, username string) (StoredAdmin, error)
	FindAdminByID(ctx context.Context, adminID string) (StoredAdmin, error)
	UpdateAdminPassword(ctx context.Context, adminID string, passwordHash string, mustChangePassword bool) error
	UpdateAdminProfile(ctx context.Context, adminID string, displayName string) (Admin, error)
	CreateAdminSession(ctx context.Context, params CreateSessionParams) error
	FindAdminBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (Session, error)
	RevokeAdminSession(ctx context.Context, tokenHash string) error
}

type Service struct {
	store      Store
	sessionTTL time.Duration
	now        func() time.Time
}

type SetupStatus struct {
	Initialized bool
	SetupOpen   bool
	AdminCount  int
}

type Admin struct {
	ID                 string
	Username           string
	DisplayName        string
	MustChangePassword bool
	Enabled            bool
}

type StoredAdmin struct {
	Admin
	PasswordHash string
}

type Session struct {
	ID        string
	TokenHash string
	Admin     Admin
	ExpiresAt time.Time
}

type SessionMeta struct {
	UserAgent string
	IPAddress string
}

type CreateFirstAdminInput struct {
	Username    string
	Password    string
	DisplayName string
}

type CreateFirstAdminParams struct {
	Username           string
	PasswordHash       string
	DisplayName        string
	MustChangePassword bool
}

type LoginInput struct {
	Username string
	Password string
	Meta     SessionMeta
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	Admin     Admin
}

type CreateSessionParams struct {
	AdminID   string
	TokenHash string
	ExpiresAt time.Time
	UserAgent string
	IPAddress string
}

type ChangePasswordInput struct {
	AdminID         string
	CurrentPassword string
	NewPassword     string
}

type UpdateProfileInput struct {
	AdminID     string
	DisplayName string
}

func NewService(store Store) Service {
	return Service{
		store:      store,
		sessionTTL: defaultSessionTTL,
		now:        time.Now,
	}
}

func (s Service) GetSetupStatus(ctx context.Context) (SetupStatus, error) {
	if s.store == nil {
		return SetupStatus{}, ErrUnauthorized
	}
	status, err := s.store.GetSetupStatus(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	status.SetupOpen = !status.Initialized && status.AdminCount == 0
	return status, nil
}

func (s Service) CreateFirstAdmin(ctx context.Context, input CreateFirstAdminInput) (Admin, error) {
	username, displayName, err := normalizeCreateAdminInput(input)
	if err != nil {
		return Admin{}, err
	}

	status, err := s.GetSetupStatus(ctx)
	if err != nil {
		return Admin{}, err
	}
	if !status.SetupOpen {
		return Admin{}, ErrSetupClosed
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return Admin{}, err
	}

	return s.store.CreateFirstAdmin(ctx, CreateFirstAdminParams{
		Username:           username,
		PasswordHash:       passwordHash,
		DisplayName:        displayName,
		MustChangePassword: true,
	})
}

func (s Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	username := normalizeUsername(input.Username)
	if username == "" || input.Password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	admin, err := s.store.FindAdminByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	if !admin.Enabled {
		return LoginResult{}, ErrInvalidCredentials
	}

	ok, err := VerifyPassword(input.Password, admin.PasswordHash)
	if err != nil || !ok {
		return LoginResult{}, ErrInvalidCredentials
	}

	token, err := NewSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	expiresAt := s.now().Add(s.sessionTTL)
	if err := s.store.CreateAdminSession(ctx, CreateSessionParams{
		AdminID:   admin.ID,
		TokenHash: HashSessionToken(token),
		ExpiresAt: expiresAt,
		UserAgent: input.Meta.UserAgent,
		IPAddress: input.Meta.IPAddress,
	}); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Token:     token,
		ExpiresAt: expiresAt,
		Admin:     admin.Admin,
	}, nil
}

func (s Service) Authenticate(ctx context.Context, token string) (Admin, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Admin{}, ErrUnauthorized
	}

	session, err := s.store.FindAdminBySessionTokenHash(ctx, HashSessionToken(token), s.now())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Admin{}, ErrUnauthorized
		}
		return Admin{}, err
	}
	if !session.Admin.Enabled {
		return Admin{}, ErrUnauthorized
	}
	return session.Admin, nil
}

func (s Service) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrUnauthorized
	}
	return s.store.RevokeAdminSession(ctx, HashSessionToken(token))
}

func (s Service) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	if strings.TrimSpace(input.AdminID) == "" || input.CurrentPassword == "" {
		return ErrInvalidCredentials
	}
	if err := validatePassword(input.NewPassword); err != nil {
		return err
	}

	admin, err := s.store.FindAdminByID(ctx, input.AdminID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrUnauthorized
		}
		return err
	}

	ok, err := VerifyPassword(input.CurrentPassword, admin.PasswordHash)
	if err != nil || !ok {
		return ErrInvalidCredentials
	}

	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	return s.store.UpdateAdminPassword(ctx, input.AdminID, passwordHash, false)
}

func (s Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (Admin, error) {
	adminID := strings.TrimSpace(input.AdminID)
	displayName := strings.TrimSpace(input.DisplayName)
	if adminID == "" || displayName == "" || len(displayName) > 64 {
		return Admin{}, ErrInvalidInput
	}
	return s.store.UpdateAdminProfile(ctx, adminID, displayName)
}

func normalizeCreateAdminInput(input CreateFirstAdminInput) (string, string, error) {
	username := normalizeUsername(input.Username)
	if len(username) < 3 || len(username) > 64 {
		return "", "", ErrInvalidInput
	}
	if err := validatePassword(input.Password); err != nil {
		return "", "", err
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if len(displayName) > 64 {
		return "", "", ErrInvalidInput
	}
	return username, displayName, nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func validatePassword(password string) error {
	if len(password) < 10 || len(password) > 128 {
		return ErrInvalidInput
	}
	return nil
}
