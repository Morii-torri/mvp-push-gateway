# Legacy Route Action Fields Cleanup Assessment

日期：2026-05-12

## 结论

不要在本轮删除 `route_actions.template_version_id` 和 `route_actions.channel_ids`。当前新模型已使用 `route_action_targets` 和 API `action.targets[]`，但 legacy 字段仍承担旧客户端兼容、历史数据回填和回滚缓冲作用。

本轮只修复新前端和 smoke 脚本，确保新写入路径使用 `action.targets[]`。

## 当前兼容路径

### 数据库

- `backend/migrations/000001_init.sql` 仍创建 `route_actions.template_version_id` 和 `route_actions.channel_ids`。
- `backend/migrations/000006_route_action_targets.sql` 创建 `route_action_targets`，并从 legacy 字段回填 target 行。
- `backend/internal/db/route.go` 保存规则时会写入 `route_action_targets`，同时写入从 targets 派生的 legacy 字段，便于旧查询和回滚。
- `backend/internal/db/route.go` 读取规则时优先加载 `route_action_targets`；如果 target 为空，使用 `legacyRouteActionTargets` 从 legacy 字段合成 target。

### API/后端服务

- `backend/internal/http/route_handlers.go` request/response 仍包含兼容字段 `template_version_id` 和 `channel_ids`。
- `backend/internal/route/service.go` 的 `normalizeActionTargets` 优先使用 `ActionInput.Targets`；仅在 targets 为空时才从 legacy 字段转换。
- `backend/internal/planning/worker.go` 执行时优先使用 `Action.Targets`；兼容逻辑只用于旧数据。
- `backend/internal/http/end_to_end_integration_test.go` 仍断言 response 中 legacy 字段可见，说明 API 兼容尚未完全移除。

### 前端

- `frontend/src/pages/console/routeRuleForm.tsx` 新保存路径 `routeRuleToInput` 只输出 `action.targets[]`。
- `routeTargetsFromApi` 仍能读取旧 response 的 `action.template_version_id + action.channel_ids`，用于展示历史规则。
- `frontend/src/api/console.ts` 的 response type 仍声明 legacy 字段；input type 不包含 legacy 字段。
- 本轮已把 `scripts/smoke-e2e.sh` 改为提交 `action.targets[]`。

### 测试/文档

仍依赖 legacy 字段的测试或示例：

- `frontend/src/pages/ConsolePages.test.tsx` 覆盖“读取 legacy action 字段并转成发送动作摘要”。
- `backend/internal/http/route_handlers_test.go` 覆盖旧 payload 仍可提交。
- `backend/internal/route/service_test.go` 覆盖旧 `template_version_id + channel_ids` 可转换为 targets。
- `backend/internal/http/end_to_end_integration_test.go` 读取 response legacy 字段。
- `docs/plans/2026-05-12-route-send-action-group-plan.md` 记录兼容期输入。

这些测试是兼容期的保护网，不应在账号联调前移除。

## 可以移除的条件

建议同时满足以下条件后再删除字段：

1. 至少一个版本周期内，新前端、新 smoke 脚本和所有新文档都只写 `action.targets[]`。
2. 真实上级账号联调完成，确认 route action group fan-out、日志详情和 retry 不依赖 legacy 字段。
3. 运维确认没有外部脚本或旧客户端仍提交 legacy payload。
4. 已完成一次生产数据巡检：所有有效 `route_actions` 都有对应 `route_action_targets`。
5. 回滚方案明确：删除字段后的版本不需要回滚到只认识 legacy 字段的后端。

## 移除前迁移步骤

1. 增加只读巡检 SQL，找出没有 target 的 action：

```sql
SELECT action.id, action.rule_id
FROM route_actions AS action
LEFT JOIN route_action_targets AS target ON target.action_id = action.id
WHERE target.id IS NULL;
```

2. 对缺失 target 的历史 action 运行一次回填迁移，逻辑等同 `000006_route_action_targets.sql`。
3. 后端 response 标记 legacy 字段 deprecated，并在一版后从 OpenAPI/前端 response type 移除。
4. 删除 `routeTargetsFromApi` 中 legacy fallback，保留数据巡检失败时的中文错误。
5. 删除后端 request legacy 字段和 `normalizeActionTargets` fallback。
6. 最后新增 DB migration 删除 `route_actions.template_version_id` 和 `route_actions.channel_ids`。

## 回滚风险

- 如果直接删除字段，旧后端或旧客户端回滚后可能无法保存/展示路由规则。
- 如果先删 API fallback，再遇到历史数据缺失 `route_action_targets`，planning 可能报“no delivery targets”。
- 如果删除前没有确认外部脚本，第三方调用仍提交 legacy payload 时会突然失败。

因此本轮保留 DB 字段和读取 fallback，只收紧新写入路径。
