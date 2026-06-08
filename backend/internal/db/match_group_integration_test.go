package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/matchgroup"
	"mvp-push-gateway/backend/internal/route"
)

func TestDeleteMatchGroupRejectsRouteRuleReferences(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name)
		VALUES ($1, $2, $3)
	`, "00000000-0000-0000-0000-000000001001", "orders", "Orders"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	group, err := repository.CreateGroup(ctx, matchgroup.CreateGroupParams{
		Name:      "来源 IP",
		GroupType: "ip",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("create match group: %v", err)
	}
	flow, err := repository.CreateFlow(ctx, route.CreateFlowParams{
		ID:       "00000000-0000-0000-0000-000000001002",
		SourceID: "00000000-0000-0000-0000-000000001001",
		Name:     "Primary",
		Enabled:  true,
		Mode:     route.ModeTable,
	})
	if err != nil {
		t.Fatalf("create route flow: %v", err)
	}
	draft, err := repository.GetDraft(ctx, flow.ID)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	_, err = repository.ReplaceRules(ctx, flow.ID, draft.Version.ID, []route.Rule{
		{
			ID:        "00000000-0000-0000-0000-000000001003",
			FlowID:    flow.ID,
			VersionID: draft.Version.ID,
			RuleKey:   "00000000-0000-0000-0000-000000001004",
			SortOrder: 1,
			Name:      "IP 命中",
			ConditionTree: json.RawMessage(`{
				"operator":"in_match_group",
				"path":"payload.ip",
				"match_group_id":"` + group.ID + `"
			}`),
			Enabled: true,
			Action:  route.Action{RecipientStrategy: json.RawMessage(`{"mode":"none"}`)},
		},
	})
	if err != nil {
		t.Fatalf("replace route rules: %v", err)
	}

	err = repository.DeleteGroup(ctx, group.ID)
	if !errors.Is(err, matchgroup.ErrInUse) {
		t.Fatalf("expected referenced match group delete to be rejected, got %v", err)
	}
}
