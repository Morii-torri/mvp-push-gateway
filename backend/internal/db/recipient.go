package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/recipient"
)

func (r Repository) ListOrgUnits(ctx context.Context) ([]recipient.OrgUnit, error) {
	rows, err := r.pool.Query(ctx, orgUnitSelectSQL()+` ORDER BY path ASC, sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list org units: %w", err)
	}
	defer rows.Close()

	items := []recipient.OrgUnit{}
	for rows.Next() {
		item, err := scanOrgUnit(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateOrgUnit(ctx context.Context, params recipient.CreateOrgUnitParams) (recipient.OrgUnit, error) {
	id := uuid.NewString()
	path := params.Code
	var parent *recipient.OrgUnit
	if params.ParentID != "" {
		loadedParent, err := r.GetOrgUnit(ctx, params.ParentID)
		if err != nil {
			return recipient.OrgUnit{}, err
		}
		parent = &loadedParent
		path = loadedParent.Path + "/" + params.Code
	}
	if err := validateOrgUnitCodeForParent(params.Code, parent); err != nil {
		return recipient.OrgUnit{}, err
	}
	item, err := r.queryOrgUnit(ctx, `
		INSERT INTO org_units (id, parent_id, code, name, sort_order, path)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6)
		RETURNING `+orgUnitSelectColumns(),
		id, params.ParentID, params.Code, params.Name, params.SortOrder, path,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return recipient.OrgUnit{}, recipient.ErrAlreadyExists
		}
		return recipient.OrgUnit{}, fmt.Errorf("create org unit: %w", err)
	}
	return item, nil
}

func (r Repository) GetOrgUnit(ctx context.Context, id string) (recipient.OrgUnit, error) {
	item, err := r.queryOrgUnit(ctx, orgUnitSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return recipient.OrgUnit{}, mapRecipientQueryError("get org unit", err)
	}
	return item, nil
}

func (r Repository) UpdateOrgUnit(ctx context.Context, id string, params recipient.UpdateOrgUnitParams) (recipient.OrgUnit, error) {
	if params.ParentID == id {
		return recipient.OrgUnit{}, recipient.ErrInvalidInput
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return recipient.OrgUnit{}, fmt.Errorf("begin update org unit transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := scanOrgUnit(tx.QueryRow(ctx, orgUnitSelectSQL()+` WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return recipient.OrgUnit{}, mapRecipientQueryError("lock org unit", err)
	}

	oldPath := current.Path
	newPath := params.Code
	var parent *recipient.OrgUnit
	if params.ParentID != "" {
		loadedParent, err := scanOrgUnit(tx.QueryRow(ctx, orgUnitSelectSQL()+` WHERE id = $1 FOR UPDATE`, params.ParentID))
		if err != nil {
			return recipient.OrgUnit{}, mapRecipientQueryError("lock parent org unit", err)
		}
		if loadedParent.Path == oldPath || strings.HasPrefix(loadedParent.Path, oldPath+"/") {
			return recipient.OrgUnit{}, recipient.ErrInvalidInput
		}
		parent = &loadedParent
		newPath = loadedParent.Path + "/" + params.Code
	}
	if err := validateOrgUnitCodeForParent(params.Code, parent); err != nil {
		return recipient.OrgUnit{}, err
	}

	item, err := scanOrgUnit(tx.QueryRow(ctx, `
		UPDATE org_units
		SET parent_id = NULLIF($2, '')::uuid,
			code = $3,
			name = $4,
			sort_order = $5,
			path = $6,
			updated_at = now()
		WHERE id = $1
		RETURNING `+orgUnitSelectColumns(),
		id, params.ParentID, params.Code, params.Name, params.SortOrder, newPath,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return recipient.OrgUnit{}, recipient.ErrAlreadyExists
		}
		return recipient.OrgUnit{}, mapRecipientQueryError("update org unit", err)
	}

	if oldPath != newPath {
		if _, err := tx.Exec(ctx, `
			UPDATE org_units
			SET path = $2 || substring(path from length($1) + 1),
				updated_at = now()
			WHERE starts_with(path, $1 || '/')
		`, oldPath, newPath); err != nil {
			return recipient.OrgUnit{}, fmt.Errorf("update descendant org paths: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return recipient.OrgUnit{}, fmt.Errorf("commit update org unit transaction: %w", err)
	}
	return item, nil
}

func validateOrgUnitCodeForParent(code string, parent *recipient.OrgUnit) error {
	level := 1
	if parent != nil {
		level = orgUnitPathDepth(parent.Path) + 1
	}
	if len(code) != orgUnitCodeLengthForLevel(level) {
		return recipient.ErrInvalidInput
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return recipient.ErrInvalidInput
		}
	}
	return nil
}

func orgUnitPathDepth(path string) int {
	depth := 0
	for _, part := range strings.Split(path, "/") {
		if part != "" {
			depth++
		}
	}
	return depth
}

func orgUnitCodeLengthForLevel(level int) int {
	if level < 1 {
		level = 1
	}
	return 4 + (level-1)*2
}

func (r Repository) DeleteOrgUnit(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return recipient.ErrInvalidInput
	}
	referenced, err := r.recipientGroupsReferenceOrg(ctx, id)
	if err != nil {
		return err
	}
	if referenced {
		return recipient.ErrInvalidInput
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM org_units WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete org unit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return recipient.ErrNotFound
	}
	return nil
}

func (r Repository) ListUsers(ctx context.Context) ([]recipient.User, error) {
	rows, err := r.pool.Query(ctx, userSelectSQL()+` ORDER BY created_at DESC, display_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	items := []recipient.User{}
	for rows.Next() {
		item, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateUser(ctx context.Context, params recipient.CreateUserParams) (recipient.User, error) {
	item, err := r.queryUser(ctx, `
		INSERT INTO users (id, display_name, primary_org_id, enabled, attributes)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5)
		RETURNING `+userSelectColumns(),
		uuid.NewString(), params.DisplayName, params.PrimaryOrgID, params.Enabled, defaultJSON(params.Attributes),
	)
	if err != nil {
		return recipient.User{}, fmt.Errorf("create user: %w", err)
	}
	return item, nil
}

func (r Repository) CreateUserProfile(ctx context.Context, params recipient.CreateUserProfileParams) (recipient.UserProfile, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return recipient.UserProfile{}, fmt.Errorf("begin create user profile transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	userID := uuid.NewString()
	user, err := scanUser(tx.QueryRow(ctx, `
		INSERT INTO users (id, display_name, primary_org_id, enabled, attributes)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5)
		RETURNING `+userSelectColumns(),
		userID, params.User.DisplayName, params.User.PrimaryOrgID, params.User.Enabled, defaultJSON(params.User.Attributes),
	))
	if err != nil {
		return recipient.UserProfile{}, mapRecipientWriteError("create user profile user", err)
	}

	for _, identity := range params.Identities {
		if err := validateUserIdentityChannelScopeTx(ctx, tx, identity.ProviderType, identity.ChannelID); err != nil {
			return recipient.UserProfile{}, err
		}
		if _, err := scanUserIdentity(tx.QueryRow(ctx, `
			INSERT INTO user_identities (id, user_id, provider_type, channel_id, identity_kind, identity_value, verified)
			VALUES ($1, $2, $3, nullif($4, '')::uuid, $5, $6, $7)
			RETURNING `+userIdentitySelectColumns(),
			uuid.NewString(), userID, identity.ProviderType, identity.ChannelID, identity.IdentityKind, identity.IdentityValue, identity.Verified,
		)); err != nil {
			return recipient.UserProfile{}, mapRecipientWriteError("create user profile identity", err)
		}
	}

	identities, err := listUserIdentitiesTx(ctx, tx, userID)
	if err != nil {
		return recipient.UserProfile{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return recipient.UserProfile{}, fmt.Errorf("commit create user profile transaction: %w", err)
	}
	return recipient.UserProfile{User: user, Identities: identities}, nil
}

func (r Repository) GetUser(ctx context.Context, id string) (recipient.User, error) {
	item, err := r.queryUser(ctx, userSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return recipient.User{}, mapRecipientQueryError("get user", err)
	}
	return item, nil
}

func (r Repository) UpdateUser(ctx context.Context, id string, params recipient.UpdateUserParams) (recipient.User, error) {
	item, err := r.queryUser(ctx, `
		UPDATE users
		SET display_name = $2,
			primary_org_id = NULLIF($3, '')::uuid,
			enabled = $4,
			attributes = $5,
			updated_at = now()
		WHERE id = $1
		RETURNING `+userSelectColumns(),
		id, params.DisplayName, params.PrimaryOrgID, params.Enabled, defaultJSON(params.Attributes),
	)
	if err != nil {
		return recipient.User{}, mapRecipientQueryError("update user", err)
	}
	return item, nil
}

func (r Repository) SaveUserProfile(ctx context.Context, id string, params recipient.SaveUserProfileParams) (recipient.UserProfile, error) {
	if _, err := uuid.Parse(id); err != nil {
		return recipient.UserProfile{}, recipient.ErrInvalidInput
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return recipient.UserProfile{}, fmt.Errorf("begin save user profile transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := scanUser(tx.QueryRow(ctx, userSelectSQL()+` WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return recipient.UserProfile{}, mapRecipientQueryError("lock user profile", err)
	}
	if params.ExpectedUpdatedAt != nil && !sameAPITime(current.UpdatedAt, *params.ExpectedUpdatedAt) {
		return recipient.UserProfile{}, recipient.ErrConflict
	}

	user, err := scanUser(tx.QueryRow(ctx, `
		UPDATE users
		SET display_name = $2,
			primary_org_id = NULLIF($3, '')::uuid,
			enabled = $4,
			attributes = $5,
			updated_at = now()
		WHERE id = $1
		RETURNING `+userSelectColumns(),
		id, params.User.DisplayName, params.User.PrimaryOrgID, params.User.Enabled, defaultJSON(params.User.Attributes),
	))
	if err != nil {
		return recipient.UserProfile{}, mapRecipientWriteError("update user profile user", err)
	}

	existingIDs, err := lockUserIdentityIDs(ctx, tx, id)
	if err != nil {
		return recipient.UserProfile{}, err
	}
	seenIDs := map[string]bool{}
	for _, identity := range params.Identities {
		if err := validateUserIdentityChannelScopeTx(ctx, tx, identity.ProviderType, identity.ChannelID); err != nil {
			return recipient.UserProfile{}, err
		}
		if identity.ID == "" {
			if _, err := scanUserIdentity(tx.QueryRow(ctx, `
				INSERT INTO user_identities (id, user_id, provider_type, channel_id, identity_kind, identity_value, verified)
				VALUES ($1, $2, $3, nullif($4, '')::uuid, $5, $6, $7)
				RETURNING `+userIdentitySelectColumns(),
				uuid.NewString(), id, identity.ProviderType, identity.ChannelID, identity.IdentityKind, identity.IdentityValue, identity.Verified,
			)); err != nil {
				return recipient.UserProfile{}, mapRecipientWriteError("create user profile identity", err)
			}
			continue
		}
		if _, err := uuid.Parse(identity.ID); err != nil {
			return recipient.UserProfile{}, recipient.ErrInvalidInput
		}
		if !existingIDs[identity.ID] || seenIDs[identity.ID] {
			return recipient.UserProfile{}, recipient.ErrInvalidInput
		}
		seenIDs[identity.ID] = true
		if _, err := scanUserIdentity(tx.QueryRow(ctx, `
			UPDATE user_identities
			SET provider_type = $2,
				channel_id = nullif($3, '')::uuid,
				identity_kind = $4,
				identity_value = $5,
				verified = $6,
				updated_at = now()
			WHERE id = $1 AND user_id = $7
			RETURNING `+userIdentitySelectColumns(),
			identity.ID, identity.ProviderType, identity.ChannelID, identity.IdentityKind, identity.IdentityValue, identity.Verified, id,
		)); err != nil {
			return recipient.UserProfile{}, mapRecipientWriteError("update user profile identity", err)
		}
	}

	removedIDs := make([]string, 0, len(existingIDs))
	for identityID := range existingIDs {
		if !seenIDs[identityID] {
			removedIDs = append(removedIDs, identityID)
		}
	}
	if len(removedIDs) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM user_identities WHERE user_id = $1 AND id = ANY($2::uuid[])`, id, removedIDs); err != nil {
			return recipient.UserProfile{}, fmt.Errorf("delete removed user profile identities: %w", err)
		}
	}

	identities, err := listUserIdentitiesTx(ctx, tx, id)
	if err != nil {
		return recipient.UserProfile{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return recipient.UserProfile{}, fmt.Errorf("commit save user profile transaction: %w", err)
	}
	return recipient.UserProfile{User: user, Identities: identities}, nil
}

func (r Repository) DeleteUser(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return recipient.ErrInvalidInput
	}
	referenced, err := r.recipientGroupsReferenceUser(ctx, id)
	if err != nil {
		return err
	}
	if referenced {
		return recipient.ErrInvalidInput
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return recipient.ErrNotFound
	}
	return nil
}

func (r Repository) ListUserIdentities(ctx context.Context, userID string) ([]recipient.UserIdentity, error) {
	rows, err := r.pool.Query(ctx, userIdentitySelectSQL()+` WHERE user_id = $1 ORDER BY provider_type, channel_id NULLS FIRST, identity_kind`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user identities: %w", err)
	}
	defer rows.Close()

	items := []recipient.UserIdentity{}
	for rows.Next() {
		item, err := scanUserIdentity(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateUserIdentity(ctx context.Context, params recipient.CreateUserIdentityParams) (recipient.UserIdentity, error) {
	if err := r.validateUserIdentityChannelScope(ctx, params.ProviderType, params.ChannelID); err != nil {
		return recipient.UserIdentity{}, err
	}
	item, err := r.queryUserIdentity(ctx, `
		INSERT INTO user_identities (id, user_id, provider_type, channel_id, identity_kind, identity_value, verified)
		VALUES ($1, $2, $3, nullif($4, '')::uuid, $5, $6, $7)
		RETURNING `+userIdentitySelectColumns(),
		uuid.NewString(), params.UserID, params.ProviderType, params.ChannelID, params.IdentityKind, params.IdentityValue, params.Verified,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return recipient.UserIdentity{}, recipient.ErrAlreadyExists
		}
		return recipient.UserIdentity{}, fmt.Errorf("create user identity: %w", err)
	}
	return item, nil
}

func (r Repository) UpdateUserIdentity(ctx context.Context, id string, params recipient.UpdateUserIdentityParams) (recipient.UserIdentity, error) {
	if err := r.validateUserIdentityChannelScope(ctx, params.ProviderType, params.ChannelID); err != nil {
		return recipient.UserIdentity{}, err
	}
	item, err := r.queryUserIdentity(ctx, `
		UPDATE user_identities
		SET user_id = $2,
			provider_type = $3,
			channel_id = nullif($4, '')::uuid,
			identity_kind = $5,
			identity_value = $6,
			verified = $7,
			updated_at = now()
		WHERE id = $1
		RETURNING `+userIdentitySelectColumns(),
		id, params.UserID, params.ProviderType, params.ChannelID, params.IdentityKind, params.IdentityValue, params.Verified,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return recipient.UserIdentity{}, recipient.ErrAlreadyExists
		}
		return recipient.UserIdentity{}, mapRecipientQueryError("update user identity", err)
	}
	return item, nil
}

func (r Repository) DeleteUserIdentity(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM user_identities WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user identity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return recipient.ErrNotFound
	}
	return nil
}

func (r Repository) FindUserIdentity(ctx context.Context, providerType string, channelID string, identityKind string, identityValue string) (recipient.UserIdentity, error) {
	item, err := r.queryUserIdentity(ctx, userIdentitySelectSQL()+`
		WHERE identity_kind = $3
			AND identity_value = $4
			AND (
				(channel_id = nullif($2, '')::uuid AND provider_type = $1)
				OR (channel_id IS NULL AND provider_type = $1)
				OR (channel_id IS NULL AND provider_type = 'common')
			)
		ORDER BY CASE
			WHEN channel_id = nullif($2, '')::uuid AND provider_type = $1 THEN 0
			WHEN channel_id IS NULL AND provider_type = $1 THEN 1
			WHEN channel_id IS NULL AND provider_type = 'common' THEN 2
			ELSE 3
		END
		LIMIT 1
	`, providerType, channelID, identityKind, identityValue)
	if err != nil {
		return recipient.UserIdentity{}, mapRecipientQueryError("find user identity", err)
	}
	return item, nil
}

func (r Repository) ListRecipientGroups(ctx context.Context) ([]recipient.RecipientGroup, error) {
	rows, err := r.pool.Query(ctx, recipientGroupSelectSQL()+` ORDER BY created_at DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list recipient groups: %w", err)
	}
	defer rows.Close()

	items := []recipient.RecipientGroup{}
	for rows.Next() {
		item, err := scanRecipientGroup(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateRecipientGroup(ctx context.Context, params recipient.CreateRecipientGroupParams) (recipient.RecipientGroup, error) {
	if err := r.validateRecipientGroupReferences(ctx, params); err != nil {
		return recipient.RecipientGroup{}, err
	}
	item, err := r.queryRecipientGroup(ctx, `
		INSERT INTO recipient_groups (id, name, user_ids, org_ids, excluded_user_ids, excluded_org_ids, enabled)
		VALUES ($1, $2, $3::uuid[], $4::uuid[], $5::uuid[], $6::uuid[], $7)
		RETURNING `+recipientGroupSelectColumns(),
		uuid.NewString(), params.Name, emptySlice(params.UserIDs), emptySlice(params.OrgIDs), emptySlice(params.ExcludedUserIDs), emptySlice(params.ExcludedOrgIDs), params.Enabled,
	)
	if err != nil {
		return recipient.RecipientGroup{}, fmt.Errorf("create recipient group: %w", err)
	}
	return item, nil
}

func (r Repository) GetRecipientGroup(ctx context.Context, id string) (recipient.RecipientGroup, error) {
	item, err := r.queryRecipientGroup(ctx, recipientGroupSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return recipient.RecipientGroup{}, mapRecipientQueryError("get recipient group", err)
	}
	return item, nil
}

func (r Repository) UpdateRecipientGroup(ctx context.Context, id string, params recipient.UpdateRecipientGroupParams) (recipient.RecipientGroup, error) {
	if err := r.validateRecipientGroupReferences(ctx, params); err != nil {
		return recipient.RecipientGroup{}, err
	}
	item, err := r.queryRecipientGroup(ctx, `
		UPDATE recipient_groups
		SET name = $2,
			user_ids = $3::uuid[],
			org_ids = $4::uuid[],
			excluded_user_ids = $5::uuid[],
			excluded_org_ids = $6::uuid[],
			enabled = $7,
			updated_at = now()
		WHERE id = $1
		RETURNING `+recipientGroupSelectColumns(),
		id, params.Name, emptySlice(params.UserIDs), emptySlice(params.OrgIDs), emptySlice(params.ExcludedUserIDs), emptySlice(params.ExcludedOrgIDs), params.Enabled,
	)
	if err != nil {
		return recipient.RecipientGroup{}, mapRecipientQueryError("update recipient group", err)
	}
	return item, nil
}

func (r Repository) DeleteRecipientGroup(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM recipient_groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete recipient group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return recipient.ErrNotFound
	}
	return nil
}

func (r Repository) queryOrgUnit(ctx context.Context, sql string, args ...any) (recipient.OrgUnit, error) {
	return scanOrgUnit(r.pool.QueryRow(ctx, sql, args...))
}

func (r Repository) queryUser(ctx context.Context, sql string, args ...any) (recipient.User, error) {
	return scanUser(r.pool.QueryRow(ctx, sql, args...))
}

func (r Repository) queryUserIdentity(ctx context.Context, sql string, args ...any) (recipient.UserIdentity, error) {
	return scanUserIdentity(r.pool.QueryRow(ctx, sql, args...))
}

func (r Repository) queryRecipientGroup(ctx context.Context, sql string, args ...any) (recipient.RecipientGroup, error) {
	return scanRecipientGroup(r.pool.QueryRow(ctx, sql, args...))
}

func (r Repository) validateUserIdentityChannelScope(ctx context.Context, providerType string, channelID string) error {
	if strings.TrimSpace(channelID) == "" {
		return nil
	}
	if _, err := uuid.Parse(channelID); err != nil {
		return recipient.ErrInvalidInput
	}
	var channelProviderType string
	if err := r.pool.QueryRow(ctx, `SELECT provider_type FROM delivery_channels WHERE id = $1`, channelID).Scan(&channelProviderType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return recipient.ErrInvalidInput
		}
		return fmt.Errorf("load identity channel scope: %w", err)
	}
	if channelProviderType != providerType {
		return recipient.ErrInvalidInput
	}
	return nil
}

func validateUserIdentityChannelScopeTx(ctx context.Context, tx pgx.Tx, providerType string, channelID string) error {
	if strings.TrimSpace(channelID) == "" {
		return nil
	}
	if _, err := uuid.Parse(channelID); err != nil {
		return recipient.ErrInvalidInput
	}
	var channelProviderType string
	if err := tx.QueryRow(ctx, `SELECT provider_type FROM delivery_channels WHERE id = $1`, channelID).Scan(&channelProviderType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return recipient.ErrInvalidInput
		}
		return fmt.Errorf("load identity channel scope: %w", err)
	}
	if channelProviderType != providerType {
		return recipient.ErrInvalidInput
	}
	return nil
}

func lockUserIdentityIDs(ctx context.Context, tx pgx.Tx, userID string) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `SELECT id::text FROM user_identities WHERE user_id = $1 FOR UPDATE`, userID)
	if err != nil {
		return nil, fmt.Errorf("lock user identities: %w", err)
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func listUserIdentitiesTx(ctx context.Context, tx pgx.Tx, userID string) ([]recipient.UserIdentity, error) {
	rows, err := tx.Query(ctx, userIdentitySelectSQL()+` WHERE user_id = $1 ORDER BY provider_type, channel_id NULLS FIRST, identity_kind`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user identities in profile transaction: %w", err)
	}
	defer rows.Close()

	items := []recipient.UserIdentity{}
	for rows.Next() {
		item, err := scanUserIdentity(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func sameAPITime(left time.Time, right time.Time) bool {
	return left.UTC().Truncate(time.Second).Equal(right.UTC().Truncate(time.Second))
}

func mapRecipientWriteError(operation string, err error) error {
	if isUniqueViolation(err) {
		return recipient.ErrAlreadyExists
	}
	return mapRecipientQueryError(operation, err)
}

func (r Repository) validateRecipientGroupReferences(ctx context.Context, params recipient.RecipientGroupInput) error {
	userIDs := uniqueStrings(append(emptySlice(params.UserIDs), params.ExcludedUserIDs...))
	orgIDs := uniqueStrings(append(emptySlice(params.OrgIDs), params.ExcludedOrgIDs...))
	if err := validateUUIDs(userIDs); err != nil {
		return err
	}
	if err := validateUUIDs(orgIDs); err != nil {
		return err
	}
	if err := r.requireExistingUsers(ctx, userIDs); err != nil {
		return err
	}
	if err := r.requireExistingOrgUnits(ctx, orgIDs); err != nil {
		return err
	}
	return nil
}

func (r Repository) requireExistingUsers(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT count(*)::integer FROM users WHERE id = ANY($1::uuid[])`, ids).Scan(&count); err != nil {
		return fmt.Errorf("validate recipient group users: %w", err)
	}
	if count != len(ids) {
		return recipient.ErrInvalidInput
	}
	return nil
}

func (r Repository) requireExistingOrgUnits(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT count(*)::integer FROM org_units WHERE id = ANY($1::uuid[])`, ids).Scan(&count); err != nil {
		return fmt.Errorf("validate recipient group org units: %w", err)
	}
	if count != len(ids) {
		return recipient.ErrInvalidInput
	}
	return nil
}

func (r Repository) recipientGroupsReferenceUser(ctx context.Context, id string) (bool, error) {
	var referenced bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM recipient_groups
			WHERE $1 = ANY(user_ids)
				OR $1 = ANY(excluded_user_ids)
		)
	`, id).Scan(&referenced); err != nil {
		return false, fmt.Errorf("check recipient group user references: %w", err)
	}
	return referenced, nil
}

func (r Repository) recipientGroupsReferenceOrg(ctx context.Context, id string) (bool, error) {
	var referenced bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM recipient_groups
			WHERE $1 = ANY(org_ids)
				OR $1 = ANY(excluded_org_ids)
		)
	`, id).Scan(&referenced); err != nil {
		return false, fmt.Errorf("check recipient group org references: %w", err)
	}
	return referenced, nil
}

func emptySlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func validateUUIDs(values []string) error {
	for _, value := range values {
		if _, err := uuid.Parse(value); err != nil {
			return recipient.ErrInvalidInput
		}
	}
	return nil
}

func scanOrgUnit(row sourceScanner) (recipient.OrgUnit, error) {
	var item recipient.OrgUnit
	if err := row.Scan(&item.ID, &item.ParentID, &item.Code, &item.Name, &item.SortOrder, &item.Path, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return recipient.OrgUnit{}, err
	}
	return item, nil
}

func scanUser(row sourceScanner) (recipient.User, error) {
	var item recipient.User
	if err := row.Scan(&item.ID, &item.DisplayName, &item.PrimaryOrgID, &item.Enabled, &item.Attributes, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return recipient.User{}, err
	}
	return item, nil
}

func scanUserIdentity(row sourceScanner) (recipient.UserIdentity, error) {
	var item recipient.UserIdentity
	if err := row.Scan(&item.ID, &item.UserID, &item.ProviderType, &item.ChannelID, &item.IdentityKind, &item.IdentityValue, &item.Verified, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return recipient.UserIdentity{}, err
	}
	return item, nil
}

func scanRecipientGroup(row sourceScanner) (recipient.RecipientGroup, error) {
	var item recipient.RecipientGroup
	if err := row.Scan(&item.ID, &item.Name, &item.UserIDs, &item.OrgIDs, &item.ExcludedUserIDs, &item.ExcludedOrgIDs, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return recipient.RecipientGroup{}, err
	}
	return item, nil
}

func orgUnitSelectSQL() string {
	return `SELECT ` + orgUnitSelectColumns() + ` FROM org_units`
}

func orgUnitSelectColumns() string {
	return `id, COALESCE(parent_id::text, ''), code, name, sort_order, path, created_at, updated_at`
}

func userSelectSQL() string {
	return `SELECT ` + userSelectColumns() + ` FROM users`
}

func userSelectColumns() string {
	return `id, display_name, COALESCE(primary_org_id::text, ''), enabled, attributes, created_at, updated_at`
}

func userIdentitySelectSQL() string {
	return `SELECT ` + userIdentitySelectColumns() + ` FROM user_identities`
}

func userIdentitySelectColumns() string {
	return `id, user_id, provider_type, COALESCE(channel_id::text, ''), identity_kind, identity_value, verified, created_at, updated_at`
}

func recipientGroupSelectSQL() string {
	return `SELECT ` + recipientGroupSelectColumns() + ` FROM recipient_groups`
}

func recipientGroupSelectColumns() string {
	return `id, name, user_ids::text[], org_ids::text[], excluded_user_ids::text[], excluded_org_ids::text[], enabled, created_at, updated_at`
}

func mapRecipientQueryError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return recipient.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
