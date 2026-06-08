package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"mvp-push-gateway/backend/internal/planning"
	"mvp-push-gateway/backend/internal/provider"

	"mvp-push-gateway/backend/internal/recipient"
)

func TestRecipientIdentityLookupAndCRUD(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	repository := NewRepository(pool)
	org, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "1000",
		Name: "运维部",
	})
	if err != nil {
		t.Fatalf("create org unit: %v", err)
	}
	user, err := repository.CreateUser(ctx, recipient.CreateUserParams{
		DisplayName:  "张三",
		PrimaryOrgID: org.ID,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	identity, err := repository.CreateUserIdentity(ctx, recipient.CreateUserIdentityParams{
		UserID:        user.ID,
		ProviderType:  "wecom_app",
		IdentityKind:  "wecom_userid",
		IdentityValue: "zhangsan",
		Verified:      true,
	})
	if err != nil {
		t.Fatalf("create identity: %v", err)
	}

	found, err := repository.FindUserIdentity(ctx, "wecom_app", "", "wecom_userid", "zhangsan")
	if err != nil {
		t.Fatalf("find identity: %v", err)
	}
	if found.ID != identity.ID || found.UserID != user.ID {
		t.Fatalf("unexpected identity lookup result: %+v", found)
	}

	updated, err := repository.UpdateUserIdentity(ctx, identity.ID, recipient.UpdateUserIdentityParams{
		UserID:        user.ID,
		ProviderType:  "wecom_app",
		IdentityKind:  "mobile",
		IdentityValue: "13800000000",
		Verified:      false,
	})
	if err != nil {
		t.Fatalf("update identity: %v", err)
	}
	if updated.IdentityKind != "mobile" || updated.IdentityValue != "13800000000" || updated.Verified {
		t.Fatalf("unexpected updated identity: %+v", updated)
	}

	if err := repository.DeleteUserIdentity(ctx, identity.ID); err != nil {
		t.Fatalf("delete identity: %v", err)
	}
	if _, err := repository.FindUserIdentity(ctx, "wecom_app", "", "mobile", "13800000000"); !errors.Is(err, recipient.ErrNotFound) {
		t.Fatalf("expected missing identity after delete, got %v", err)
	}
}

func TestResolveSystemRecipientsUsesIdentityPriorityAndUserAttributeFallback(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderEmail,
		Name:             "邮件渠道",
		Enabled:          true,
		SendConfig:       json.RawMessage(`{}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":1}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create email channel: %v", err)
	}

	userWithChannel := createRecipientTestUser(t, ctx, repository, "渠道优先", `{"email":"channel-base@example.com"}`)
	createRecipientTestIdentity(t, ctx, repository, userWithChannel.ID, "common", "", "email", "channel-common@example.com")
	createRecipientTestIdentity(t, ctx, repository, userWithChannel.ID, "email", "", "email", "channel-provider@example.com")
	createRecipientTestIdentity(t, ctx, repository, userWithChannel.ID, "email", channel.ID, "email", "channel-specific@example.com")

	userWithProvider := createRecipientTestUser(t, ctx, repository, "平台优先", `{"email":"provider-base@example.com"}`)
	createRecipientTestIdentity(t, ctx, repository, userWithProvider.ID, "common", "", "email", "provider-common@example.com")
	createRecipientTestIdentity(t, ctx, repository, userWithProvider.ID, "email", "", "email", "provider-default@example.com")

	userWithCommon := createRecipientTestUser(t, ctx, repository, "通用兜底", `{"email":"common-base@example.com"}`)
	createRecipientTestIdentity(t, ctx, repository, userWithCommon.ID, "common", "", "email", "common-default@example.com")

	userWithAttribute := createRecipientTestUser(t, ctx, repository, "基础字段兜底", `{"email":"attribute@example.com"}`)
	disabledUser := createRecipientTestUser(t, ctx, repository, "停用用户", `{"email":"disabled-base@example.com"}`)
	if _, err := repository.UpdateUser(ctx, disabledUser.ID, recipient.UpdateUserParams{
		DisplayName:  disabledUser.DisplayName,
		PrimaryOrgID: disabledUser.PrimaryOrgID,
		Enabled:      false,
		Attributes:   disabledUser.Attributes,
	}); err != nil {
		t.Fatalf("disable test user: %v", err)
	}
	createRecipientTestIdentity(t, ctx, repository, disabledUser.ID, "email", "", "email", "disabled@example.com")

	values, err := repository.ResolveSystemRecipients(ctx, planning.ResolveSystemRecipientsParams{
		ProviderType: provider.ProviderEmail,
		ChannelID:    channel.ID,
		IdentityKind: "email",
		UserIDs:      []string{userWithChannel.ID, userWithProvider.ID, userWithCommon.ID, userWithAttribute.ID, disabledUser.ID},
	})
	if err != nil {
		t.Fatalf("resolve system recipients: %v", err)
	}
	expected := []string{"attribute@example.com", "channel-specific@example.com", "common-default@example.com", "provider-default@example.com"}
	if !equalStrings(values, expected) {
		t.Fatalf("expected %v, got %v", expected, values)
	}
}

func TestFindUserIdentityFallsBackToCommonDefault(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	user := createRecipientTestUser(t, ctx, repository, "通用身份用户", `{}`)
	common := createRecipientTestIdentity(t, ctx, repository, user.ID, "common", "", "email", "common-lookup@example.com")

	found, err := repository.FindUserIdentity(ctx, "email", "", "email", "common-lookup@example.com")
	if err != nil {
		t.Fatalf("find common fallback identity: %v", err)
	}
	if found.ID != common.ID {
		t.Fatalf("expected common identity %s, got %+v", common.ID, found)
	}
}

func TestRecipientGroupValidatesMembersAndDeleteReferences(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	missingUserID := testUUID(25001)
	if _, err := repository.CreateRecipientGroup(ctx, recipient.CreateRecipientGroupParams{
		Name:            "不存在用户组",
		UserIDs:         []string{missingUserID},
		OrgIDs:          []string{},
		ExcludedUserIDs: []string{},
		ExcludedOrgIDs:  []string{},
		Enabled:         true,
	}); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected missing user to be rejected, got %v", err)
	}

	org, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "1000",
		Name: "运维引用组",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	user, err := repository.CreateUser(ctx, recipient.CreateUserParams{
		DisplayName:  "引用用户",
		PrimaryOrgID: org.ID,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := repository.CreateRecipientGroup(ctx, recipient.CreateRecipientGroupParams{
		Name:            "引用保护组",
		UserIDs:         []string{user.ID},
		OrgIDs:          []string{org.ID},
		ExcludedUserIDs: []string{},
		ExcludedOrgIDs:  []string{},
		Enabled:         true,
	}); err != nil {
		t.Fatalf("create recipient group: %v", err)
	}
	if err := repository.DeleteUser(ctx, user.ID); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected referenced user delete to be rejected, got %v", err)
	}
	if err := repository.DeleteOrgUnit(ctx, org.ID); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected referenced org delete to be rejected, got %v", err)
	}
}

func TestCreateUserIdentityRejectsChannelProviderMismatch(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderWebhook,
		Name:             "Webhook 渠道",
		Enabled:          true,
		SendConfig:       json.RawMessage(`{}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":1}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	user := createRecipientTestUser(t, ctx, repository, "身份校验用户", `{}`)

	_, err = repository.CreateUserIdentity(ctx, recipient.CreateUserIdentityParams{
		UserID:        user.ID,
		ProviderType:  "email",
		ChannelID:     channel.ID,
		IdentityKind:  "email",
		IdentityValue: "mismatch@example.com",
	})
	if !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected mismatched channel provider to be rejected, got %v", err)
	}
}

func TestRepositorySaveUserProfileReplacesIdentitiesAtomically(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	user := createRecipientTestUser(t, ctx, repository, "旧姓名", `{"mobile":"13800000000"}`)
	kept := createRecipientTestIdentity(t, ctx, repository, user.ID, "common", "", "mobile", "13800000000")
	removed := createRecipientTestIdentity(t, ctx, repository, user.ID, "email", "", "email", "old@example.com")

	profile, err := repository.SaveUserProfile(ctx, user.ID, recipient.SaveUserProfileParams{
		User: recipient.UpdateUserParams{
			DisplayName: "新姓名",
			Enabled:     false,
			Attributes:  json.RawMessage(`{"mobile":"13900000000","email":"new@example.com"}`),
		},
		Identities: []recipient.UserProfileIdentityParams{
			{
				ID:            kept.ID,
				ProviderType:  "common",
				IdentityKind:  "mobile",
				IdentityValue: "13900000000",
				Verified:      true,
			},
			{
				ProviderType:  "email",
				IdentityKind:  "email",
				IdentityValue: "new@example.com",
				Verified:      false,
			},
		},
		ExpectedUpdatedAt: &user.UpdatedAt,
	})
	if err != nil {
		t.Fatalf("save user profile: %v", err)
	}
	if profile.User.DisplayName != "新姓名" || profile.User.Enabled {
		t.Fatalf("unexpected saved user: %+v", profile.User)
	}
	if len(profile.Identities) != 2 {
		t.Fatalf("expected two identities after replace, got %+v", profile.Identities)
	}
	if _, err := repository.GetUser(ctx, user.ID); err != nil {
		t.Fatalf("reload saved user: %v", err)
	}
	if _, err := repository.FindUserIdentity(ctx, "email", "", "email", "old@example.com"); !errors.Is(err, recipient.ErrNotFound) {
		t.Fatalf("expected omitted identity %s to be deleted, got %v", removed.ID, err)
	}
	found, err := repository.FindUserIdentity(ctx, "common", "", "mobile", "13900000000")
	if err != nil {
		t.Fatalf("find updated identity: %v", err)
	}
	if found.ID != kept.ID || found.UserID != user.ID {
		t.Fatalf("expected existing identity to be updated in place, got %+v", found)
	}
}

func TestRepositorySaveUserProfileRollsBackWhenIdentityInvalid(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderWebhook,
		Name:             "Webhook 渠道",
		Enabled:          true,
		SendConfig:       json.RawMessage(`{}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":1}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	user := createRecipientTestUser(t, ctx, repository, "回滚前", `{}`)

	_, err = repository.SaveUserProfile(ctx, user.ID, recipient.SaveUserProfileParams{
		User: recipient.UpdateUserParams{
			DisplayName: "不应保存",
			Enabled:     false,
			Attributes:  json.RawMessage(`{"email":"bad@example.com"}`),
		},
		Identities: []recipient.UserProfileIdentityParams{
			{
				ProviderType:  "email",
				ChannelID:     channel.ID,
				IdentityKind:  "email",
				IdentityValue: "bad@example.com",
				Verified:      true,
			},
		},
		ExpectedUpdatedAt: &user.UpdatedAt,
	})
	if !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected invalid profile identity to be rejected, got %v", err)
	}

	reloaded, err := repository.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("reload user after failed profile save: %v", err)
	}
	if reloaded.DisplayName != "回滚前" || !reloaded.Enabled {
		t.Fatalf("expected user update to roll back, got %+v", reloaded)
	}
	identities, err := repository.ListUserIdentities(ctx, user.ID)
	if err != nil {
		t.Fatalf("list identities after failed profile save: %v", err)
	}
	if len(identities) != 0 {
		t.Fatalf("expected no identities after failed profile save, got %+v", identities)
	}
}

func TestOrgUnitCodeLengthFollowsHierarchy(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	if _, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "100001",
		Name: "错误根组织",
	}); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected six-digit root code to be rejected, got %v", err)
	}

	root, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "1000",
		Name: "根组织",
	})
	if err != nil {
		t.Fatalf("create root org: %v", err)
	}
	if _, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: root.ID,
		Code:     "1000",
		Name:     "错误子组织",
	}); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected root-length child code to be rejected, got %v", err)
	}
	child, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: root.ID,
		Code:     "100001",
		Name:     "子组织",
	})
	if err != nil {
		t.Fatalf("create child org: %v", err)
	}
	if _, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: child.ID,
		Code:     "100001",
		Name:     "错误孙组织",
	}); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected child-length grandchild code to be rejected, got %v", err)
	}
	if _, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: child.ID,
		Code:     "10000101",
		Name:     "孙组织",
	}); err != nil {
		t.Fatalf("create grandchild org: %v", err)
	}
}

func TestUpdateOrgUnitMoveRefreshesDescendantPaths(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository, cleanup := newRecipientIntegrationRepository(ctx, t, dsn)
	defer cleanup()

	root, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "1000",
		Name: "根组织",
	})
	if err != nil {
		t.Fatalf("create root org: %v", err)
	}
	child, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: root.ID,
		Code:     "100001",
		Name:     "子组织",
	})
	if err != nil {
		t.Fatalf("create child org: %v", err)
	}
	grandchild, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: child.ID,
		Code:     "10000101",
		Name:     "孙组织",
	})
	if err != nil {
		t.Fatalf("create grandchild org: %v", err)
	}
	newParent, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "1001",
		Name: "新父组织",
	})
	if err != nil {
		t.Fatalf("create new parent org: %v", err)
	}

	moved, err := repository.UpdateOrgUnit(ctx, child.ID, recipient.UpdateOrgUnitParams{
		ParentID: newParent.ID,
		Code:     "100101",
		Name:     "子组织",
	})
	if err != nil {
		t.Fatalf("move child org: %v", err)
	}
	if moved.Path != "1001/100101" {
		t.Fatalf("expected moved child path 1001/100101, got %q", moved.Path)
	}

	reloadedGrandchild, err := repository.GetOrgUnit(ctx, grandchild.ID)
	if err != nil {
		t.Fatalf("reload grandchild org: %v", err)
	}
	if reloadedGrandchild.Path != "1001/100101/10000101" {
		t.Fatalf("expected descendant path 1001/100101/10000101, got %q", reloadedGrandchild.Path)
	}
}

func TestUpdateOrgUnitRejectsMovingToSelfOrDescendant(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository, cleanup := newRecipientIntegrationRepository(ctx, t, dsn)
	defer cleanup()

	root, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "1000",
		Name: "根组织",
	})
	if err != nil {
		t.Fatalf("create root org: %v", err)
	}
	child, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: root.ID,
		Code:     "100001",
		Name:     "子组织",
	})
	if err != nil {
		t.Fatalf("create child org: %v", err)
	}
	grandchild, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		ParentID: child.ID,
		Code:     "10000101",
		Name:     "孙组织",
	})
	if err != nil {
		t.Fatalf("create grandchild org: %v", err)
	}

	if _, err := repository.UpdateOrgUnit(ctx, child.ID, recipient.UpdateOrgUnitParams{
		ParentID: child.ID,
		Code:     "100001",
		Name:     "子组织",
	}); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected moving org to itself to return ErrInvalidInput, got %v", err)
	}

	if _, err := repository.UpdateOrgUnit(ctx, child.ID, recipient.UpdateOrgUnitParams{
		ParentID: grandchild.ID,
		Code:     "1000010101",
		Name:     "子组织",
	}); !errors.Is(err, recipient.ErrInvalidInput) {
		t.Fatalf("expected moving org under descendant to return ErrInvalidInput, got %v", err)
	}
}

func newRecipientIntegrationRepository(ctx context.Context, t *testing.T, dsn string) (Repository, func()) {
	t.Helper()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		dropTestSchema(schemaName)
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		dropTestSchema(schemaName)
		t.Fatalf("open test pool: %v", err)
	}

	cleanup := func() {
		pool.Close()
		dropTestSchema(schemaName)
	}
	return NewRepository(pool), cleanup
}

func createRecipientTestUser(t *testing.T, ctx context.Context, repository Repository, name string, attributes string) recipient.User {
	t.Helper()
	user, err := repository.CreateUser(ctx, recipient.CreateUserParams{
		DisplayName: name,
		Enabled:     true,
		Attributes:  json.RawMessage(attributes),
	})
	if err != nil {
		t.Fatalf("create recipient test user %q: %v", name, err)
	}
	return user
}

func createRecipientTestIdentity(t *testing.T, ctx context.Context, repository Repository, userID string, providerType string, channelID string, identityKind string, identityValue string) recipient.UserIdentity {
	t.Helper()
	identity, err := repository.CreateUserIdentity(ctx, recipient.CreateUserIdentityParams{
		UserID:        userID,
		ProviderType:  providerType,
		ChannelID:     channelID,
		IdentityKind:  identityKind,
		IdentityValue: identityValue,
		Verified:      true,
	})
	if err != nil {
		t.Fatalf("create recipient test identity %q/%q/%q: %v", providerType, identityKind, identityValue, err)
	}
	return identity
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
