# Product Convergence Implementation Plan

**Goal:** 完成 route send action group 手动 UI smoke、test-send dry-run 边界、操作员指南、菜单收敛和 legacy route action 字段清理评估。

**Architecture:** 后端保持真实发送 API 的显式 `send=true` 边界，前端默认只调用 build-request/dry-run 并把真实发送入口置于风险确认之后。管理台用七个一级菜单承载当前能力，旧页面 key 保留为兼容入口。文档明确模板只管消息内容、接收人在路由策略处理、多数 provider 当前仅 build-request/mock。

**Tech Stack:** Go backend, PostgreSQL, React + Vite + TypeScript + Ant Design, existing docs under `docs/`.

---

## Task 1: test-send 边界

**Files:**

- Modify: `backend/internal/provider/service.go`
- Modify: `backend/internal/http/provider_handlers.go`
- Modify: `frontend/src/pages/console/providerConfig.tsx`
- Test: `backend/internal/provider/service_test.go`
- Test: `backend/internal/http/console_missing_handlers_test.go`
- Test: `frontend/src/pages/ConsolePages.test.tsx`

**Steps:**

1. 先写测试：dry-run 返回 `final_request`、`target_context`、`rendered_message`、`resolved_recipients`，并在 `send=false` 时不产生 response。
2. 先写测试：真实发送必须显式 `send=true` 且前端出现中文风险提示、二次确认文案。
3. 实现后端 dry-run 结果结构，缺少 URL、凭证、接收人或必要配置时返回中文错误。
4. 实现前端默认按钮“生成 dry-run 请求”，真实发送使用独立危险按钮和确认弹窗。

## Task 2: 菜单/页面合并

**Files:**

- Modify: `frontend/src/app/navigation.tsx`
- Modify: `frontend/src/app/AppShell.tsx`
- Modify: `frontend/src/pages/ConsolePages.tsx`
- Test: `frontend/src/app/navigation.test.ts`
- Test: `frontend/src/pages/ConsolePages.test.tsx`

**Steps:**

1. 先写测试：主菜单是七项；旧 key 仍可解析；前端不显示旧术语。
2. 新增“日志与监控”页面 tabs：消息日志、队列监控、操作审计。
3. 新增“路由策略”页面 tabs：路由大组、匹配组、接收人组。
4. 新增“系统设置”页面 tabs：系统参数、组织人员。
5. 保留旧页面导出和 lazy loader。

## Task 3: legacy 字段评估与新前端输出

**Files:**

- Modify: `frontend/src/pages/console/routeRuleForm.tsx`
- Modify: `scripts/smoke-e2e.sh`
- Add: `docs/plans/2026-05-12-legacy-route-action-fields-cleanup-assessment.md`

**Steps:**

1. 确认 `routeRuleToInput` 只输出 `action.targets[]`。
2. 把 smoke 脚本从 legacy `template_version_id/channel_ids` 改为 `targets[]`。
3. 写评估文档：兼容路径、依赖点、移除时机、迁移步骤、回滚风险。

## Task 4: 操作员指南与 smoke 记录

**Files:**

- Add: `docs/operations/operator-guide.md`
- Add: `docs/operations/2026-05-12-route-send-action-group-ui-smoke.md`
- Modify: `docs/README.md`
- Modify: `docs/operations/end-to-end-smoke.md`

**Steps:**

1. 重写操作员指南，覆盖来源接入、推送渠道、消息模板、路由策略、发送动作组 target、接收人策略、入站测试、日志排查。
2. 明确模板不写接收人，接收人在路由策略处理。
3. 明确 provider 多数是 build-request/mock，等待账号联调。
4. 记录真实本地 UI smoke 步骤、测试数据、结果和截图路径。

## Task 5: 验证

**Commands:**

- `cd backend && set -a; source ../.env; set +a; go test -mod=readonly ./... -count=1`
- `cd frontend && npm run build && npm test -- --run`
- `./scripts/check-shell-scripts.sh`
- `docker compose config`
- `docker compose --profile all-in-one config`

**Manual UI smoke:**

启动真实 PostgreSQL、真实后端、真实前端和本地 fake webhook。通过管理台创建来源、两个不同 provider type 渠道实例、两个兼容模板版本、一个含两个 action targets 的路由规则，发布激活后发送入站 payload，并验证 planning fan-out、delivery attempts 和消息日志详情。
