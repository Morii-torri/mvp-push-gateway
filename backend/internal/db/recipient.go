package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	if params.ParentID != "" {
		parent, err := r.GetOrgUnit(ctx, params.ParentID)
		if err != nil {
			return recipient.OrgUnit{}, err
		}
		path = parent.Path + "/" + params.Code
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
	if params.ParentID != "" {
		parent, err := scanOrgUnit(tx.QueryRow(ctx, orgUnitSelectSQL()+` WHERE id = $1 FOR UPDATE`, params.ParentID))
		if err != nil {
			return recipient.OrgUnit{}, mapRecipientQueryError("lock parent org unit", err)
		}
		if parent.Path == oldPath || strings.HasPrefix(parent.Path, oldPath+"/") {
			return recipient.OrgUnit{}, recipient.ErrInvalidInput
		}
		newPath = parent.Path + "/" + params.Code
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

func (r Repository) DeleteOrgUnit(ctx context.Context, id string) error {
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

func (r Repository) DeleteUser(ctx context.Context, id string) error {
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
	rows, err := r.pool.Query(ctx, userIdentitySelectSQL()+` WHERE user_id = $1 ORDER BY provider_type, identity_kind`, userID)
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
	item, err := r.queryUserIdentity(ctx, `
		INSERT INTO user_identities (id, user_id, provider_type, identity_kind, identity_value, verified)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+userIdentitySelectColumns(),
		uuid.NewString(), params.UserID, params.ProviderType, params.IdentityKind, params.IdentityValue, params.Verified,
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
	item, err := r.queryUserIdentity(ctx, `
		UPDATE user_identities
		SET user_id = $2,
			provider_type = $3,
			identity_kind = $4,
			identity_value = $5,
			verified = $6,
			updated_at = now()
		WHERE id = $1
		RETURNING `+userIdentitySelectColumns(),
		id, params.UserID, params.ProviderType, params.IdentityKind, params.IdentityValue, params.Verified,
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

func (r Repository) FindUserIdentity(ctx context.Context, providerType string, identityKind string, identityValue string) (recipient.UserIdentity, error) {
	item, err := r.queryUserIdentity(ctx, userIdentitySelectSQL()+`
		WHERE provider_type = $1 AND identity_kind = $2 AND identity_value = $3
	`, providerType, identityKind, identityValue)
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
	item, err := r.queryRecipientGroup(ctx, `
		INSERT INTO recipient_groups (id, name, user_ids, org_ids, excluded_user_ids, excluded_org_ids, enabled)
		VALUES ($1, $2, $3::uuid[], $4::uuid[], $5::uuid[], $6::uuid[], $7)
		RETURNING `+recipientGroupSelectColumns(),
		uuid.NewString(), params.Name, params.UserIDs, params.OrgIDs, params.ExcludedUserIDs, params.ExcludedOrgIDs, params.Enabled,
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
		id, params.Name, params.UserIDs, params.OrgIDs, params.ExcludedUserIDs, params.ExcludedOrgIDs, params.Enabled,
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
	if err := row.Scan(&item.ID, &item.UserID, &item.ProviderType, &item.IdentityKind, &item.IdentityValue, &item.Verified, &item.CreatedAt, &item.UpdatedAt); err != nil {
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
	return `id, user_id, provider_type, identity_kind, identity_value, verified, created_at, updated_at`
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
