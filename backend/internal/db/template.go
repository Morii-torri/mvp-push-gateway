package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func (r Repository) ListTemplates(ctx context.Context) ([]msgtemplate.Template, error) {
	rows, err := r.pool.Query(ctx, templateSelectSQL()+` ORDER BY created_at DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	items := []msgtemplate.Template{}
	for rows.Next() {
		item, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		if item.CurrentVersionID != "" {
			version, err := r.getTemplateVersion(ctx, item.CurrentVersionID)
			if err != nil {
				return nil, err
			}
			item.CurrentVersion = &version
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateTemplate(ctx context.Context, params msgtemplate.CreateTemplateParams) (msgtemplate.Template, error) {
	item, err := r.queryTemplate(ctx, `
		INSERT INTO templates (id, name, description, source_id, enabled)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5)
		RETURNING `+templateSelectColumns(),
		uuid.NewString(), params.Name, params.Description, params.SourceID, params.Enabled,
	)
	if err != nil {
		return msgtemplate.Template{}, fmt.Errorf("create template: %w", err)
	}
	return item, nil
}

func (r Repository) GetTemplate(ctx context.Context, id string) (msgtemplate.Template, error) {
	item, err := r.queryTemplate(ctx, templateSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return msgtemplate.Template{}, mapTemplateQueryError("get template", err)
	}
	return item, nil
}

func (r Repository) UpdateTemplate(ctx context.Context, id string, params msgtemplate.UpdateTemplateParams) (msgtemplate.Template, error) {
	item, err := r.queryTemplate(ctx, `
		UPDATE templates
		SET name = $2,
			description = $3,
			source_id = NULLIF($4, '')::uuid,
			enabled = $5,
			updated_at = now()
		WHERE id = $1
		RETURNING `+templateSelectColumns(),
		id, params.Name, params.Description, params.SourceID, params.Enabled,
	)
	if err != nil {
		return msgtemplate.Template{}, mapTemplateQueryError("update template", err)
	}
	return item, nil
}

func (r Repository) DeleteTemplate(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return msgtemplate.ErrNotFound
	}
	return nil
}

func (r Repository) ListTemplateVersions(ctx context.Context, templateID string) ([]msgtemplate.TemplateVersion, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT true FROM templates WHERE id = $1`, templateID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, msgtemplate.ErrNotFound
		}
		return nil, fmt.Errorf("check template before list versions: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+templateVersionSelectColumns()+`
		FROM template_versions
		WHERE template_id = $1
		ORDER BY version_no DESC
	`, templateID)
	if err != nil {
		return nil, fmt.Errorf("list template versions: %w", err)
	}
	defer rows.Close()

	versions := []msgtemplate.TemplateVersion{}
	for rows.Next() {
		version, err := scanTemplateVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list template versions rows: %w", err)
	}
	return versions, nil
}

func (r Repository) GetTemplateVersionForRestore(ctx context.Context, templateID string, versionID string) (msgtemplate.TemplateVersion, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+templateVersionSelectColumns()+`
		FROM template_versions
		WHERE template_id = $1
			AND id = $2
	`, templateID, versionID)
	version, err := scanTemplateVersion(row)
	if err != nil {
		return msgtemplate.TemplateVersion{}, mapTemplateQueryError("get template version", err)
	}
	return version, nil
}

func (r Repository) PublishTemplateVersion(ctx context.Context, templateID string, params msgtemplate.PublishTemplateVersionParams) (msgtemplate.TemplateVersion, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return msgtemplate.TemplateVersion{}, fmt.Errorf("begin publish template version transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM templates WHERE id = $1 FOR UPDATE`, templateID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return msgtemplate.TemplateVersion{}, msgtemplate.ErrNotFound
		}
		return msgtemplate.TemplateVersion{}, fmt.Errorf("lock template: %w", err)
	}

	var versionNo int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(version_no), 0) + 1 FROM template_versions WHERE template_id = $1`, templateID).Scan(&versionNo); err != nil {
		return msgtemplate.TemplateVersion{}, fmt.Errorf("query next template version: %w", err)
	}

	versionID := uuid.NewString()
	row := tx.QueryRow(ctx, `
		INSERT INTO template_versions (
			id,
			template_id,
			version_no,
			message_type,
			target_provider_type,
			template_engine,
			template_syntax_version,
			template_body,
			message_body_schema,
			sample_payload,
			compiled_preview,
			used_variables,
			validation_status,
			validation_errors,
			published_at
		)
		VALUES ($1, $2, $3, $4, $5, 'pongo2', 'jinja-like-v1', $6, $7, $8, $9, $10, $11, $12, now())
		RETURNING `+templateVersionSelectColumns(),
		versionID,
		templateID,
		versionNo,
		params.MessageType,
		params.TargetProviderType,
		params.TemplateBody,
		defaultJSON(params.MessageBodySchema),
		defaultJSON(params.SamplePayload),
		params.CompiledPreview,
		params.UsedVariables,
		params.ValidationStatus,
		params.ValidationErrors,
	)
	version, err := scanTemplateVersion(row)
	if err != nil {
		return msgtemplate.TemplateVersion{}, fmt.Errorf("insert template version: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE templates
		SET current_version_id = $2,
			updated_at = now()
		WHERE id = $1
	`, templateID, version.ID); err != nil {
		return msgtemplate.TemplateVersion{}, fmt.Errorf("update current template version: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE route_action_targets
		SET template_version_id = $2
		WHERE template_version_id IN (
			SELECT id FROM template_versions WHERE template_id = $1
		)
	`, templateID, version.ID); err != nil {
		return msgtemplate.TemplateVersion{}, fmt.Errorf("update route action target template versions: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE route_actions
		SET template_version_id = $2
		WHERE template_version_id IN (
			SELECT id FROM template_versions WHERE template_id = $1
		)
	`, templateID, version.ID); err != nil {
		return msgtemplate.TemplateVersion{}, fmt.Errorf("update route action template versions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return msgtemplate.TemplateVersion{}, fmt.Errorf("commit publish template version transaction: %w", err)
	}
	return version, nil
}

func (r Repository) queryTemplate(ctx context.Context, sql string, args ...any) (msgtemplate.Template, error) {
	return scanTemplate(r.pool.QueryRow(ctx, sql, args...))
}

func scanTemplate(row sourceScanner) (msgtemplate.Template, error) {
	var item msgtemplate.Template
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Description,
		&item.SourceID,
		&item.Enabled,
		&item.CurrentVersionID,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return msgtemplate.Template{}, err
	}
	return item, nil
}

func scanTemplateVersion(row sourceScanner) (msgtemplate.TemplateVersion, error) {
	var item msgtemplate.TemplateVersion
	var publishedAt pgtype.Timestamptz
	if err := row.Scan(
		&item.ID,
		&item.TemplateID,
		&item.VersionNo,
		&item.MessageType,
		&item.TargetProviderType,
		&item.TemplateEngine,
		&item.TemplateSyntaxVersion,
		&item.TemplateBody,
		&item.MessageBodySchema,
		&item.SamplePayload,
		&item.CompiledPreview,
		&item.UsedVariables,
		&item.AllowedFilters,
		&item.ValidationStatus,
		&item.ValidationErrors,
		&publishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return msgtemplate.TemplateVersion{}, err
	}
	if publishedAt.Valid {
		item.PublishedAt = &publishedAt.Time
	}
	return item, nil
}

func (r Repository) getTemplateVersion(ctx context.Context, id string) (msgtemplate.TemplateVersion, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+templateVersionSelectColumns()+` FROM template_versions WHERE id = $1`, id)
	item, err := scanTemplateVersion(row)
	if err != nil {
		return msgtemplate.TemplateVersion{}, mapTemplateQueryError("get template version", err)
	}
	return item, nil
}

func templateSelectSQL() string {
	return `SELECT ` + templateSelectColumns() + ` FROM templates`
}

func templateSelectColumns() string {
	return `id, name, description, COALESCE(source_id::text, ''), enabled, COALESCE(current_version_id::text, ''), created_at, updated_at`
}

func templateVersionSelectColumns() string {
	return `
		id,
		template_id,
		version_no,
		message_type,
		target_provider_type,
		template_engine,
		template_syntax_version,
		template_body,
		message_body_schema,
		sample_payload,
		compiled_preview,
		used_variables,
		allowed_filters,
		validation_status,
		validation_errors,
		published_at,
		created_at,
		updated_at
	`
}

func mapTemplateQueryError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return msgtemplate.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
