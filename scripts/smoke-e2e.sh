#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/load-env.sh"
load_project_env "$ROOT_DIR"

API_BASE="${MGP_SMOKE_API_BASE:-http://127.0.0.1:${MGP_PORT:-18080}/api/v1}"
ADMIN_USERNAME="${MGP_SMOKE_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${MGP_SMOKE_ADMIN_PASSWORD:-ChangeMe-Smoke-12345}"
ADMIN_DISPLAY_NAME="${MGP_SMOKE_ADMIN_DISPLAY_NAME:-Smoke Admin}"
WEBHOOK_URL="${MGP_SMOKE_WEBHOOK_URL:-http://127.0.0.1:18081/webhook}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

json_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token="${4:-}"
  local tmp_body tmp_status
  tmp_body="$(mktemp)"
  tmp_status="$(mktemp)"
  local args=(-sS -X "$method" "$API_BASE$path" -H "Content-Type: application/json" -o "$tmp_body" -w "%{http_code}")
  if [[ -n "$token" ]]; then
    args+=(-H "Authorization: Bearer $token")
  fi
  if [[ -n "$body" ]]; then
    args+=(-d "$body")
  fi
  curl "${args[@]}" > "$tmp_status"
  local status
  status="$(cat "$tmp_status")"
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    echo "request failed: $method $path -> $status" >&2
    cat "$tmp_body" >&2
    rm -f "$tmp_body" "$tmp_status"
    exit 1
  fi
  cat "$tmp_body"
  rm -f "$tmp_body" "$tmp_status"
}

require_cmd curl
require_cmd jq

setup_status="$(json_request GET /setup/status)"
if [[ "$(jq -r '.setup_open' <<<"$setup_status")" == "true" ]]; then
  json_request POST /setup/admin "$(jq -n \
    --arg username "$ADMIN_USERNAME" \
    --arg password "$ADMIN_PASSWORD" \
    --arg display_name "$ADMIN_DISPLAY_NAME" \
    '{username:$username,password:$password,display_name:$display_name}')" >/dev/null
  echo "created first admin: $ADMIN_USERNAME"
fi

login_response="$(json_request POST /auth/login "$(jq -n \
  --arg username "$ADMIN_USERNAME" \
  --arg password "$ADMIN_PASSWORD" \
  '{username:$username,password:$password}')")"
admin_token="$(jq -r '.token' <<<"$login_response")"
echo "logged in as $ADMIN_USERNAME"

suffix="$(date +%s)"
source_code="smoke${suffix}"
source_token="smoketoken${suffix}"
cascade_source_code="cascade${suffix}"
cascade_source_token="cascadetoken${suffix}"

source_response="$(json_request POST /sources "$(jq -n \
  --arg code "$source_code" \
  --arg token "$source_token" \
  '{
    code:$code,
    name:"Smoke 来源",
    enabled:true,
    auth_mode:"token",
    auth_token:$token,
    compat_mode:"standard",
    inbound_dedupe_enabled:false,
    inbound_dedupe_strategy:"payload_hash",
    inbound_dedupe_config:{},
    rate_limit_config:{},
    ip_allowlist:[]
  }')" "$admin_token")"
source_id="$(jq -r '.source.id' <<<"$source_response")"
echo "created source: $source_code"

cascade_source_response="$(json_request POST /sources "$(jq -n \
  --arg code "$cascade_source_code" \
  --arg token "$cascade_source_token" \
  '{
    code:$code,
    name:"Smoke 级联来源",
    enabled:true,
    auth_mode:"token",
    auth_token:$token,
    compat_mode:"standard",
    inbound_dedupe_enabled:false,
    inbound_dedupe_strategy:"payload_hash",
    inbound_dedupe_config:{},
    rate_limit_config:{},
    ip_allowlist:[]
  }')" "$admin_token")"
cascade_source_id="$(jq -r '.source.id' <<<"$cascade_source_response")"
echo "created cascade source: $cascade_source_code ($cascade_source_id)"

webhook_channel_response="$(json_request POST /channels "$(jq -n \
  --arg url "$WEBHOOK_URL" \
  '{
    provider_type:"webhook",
    name:"Smoke Webhook",
    enabled:true,
    auth_config:{},
    token_config:{},
    send_config:{
      method:"POST",
      url:$url,
      headers:{"Content-Type":"application/json"},
      body:{gateway:"mvp-push"},
      recipient:{location:"none"}
    },
    rate_limit_config:{},
    concurrency_limit:2,
    timeout_ms:5000,
    retry_policy:{max_attempts:2},
    dead_letter_policy:{enabled:true}
  }')" "$admin_token")"
webhook_channel_id="$(jq -r '.channel.id' <<<"$webhook_channel_response")"
echo "created webhook channel: $webhook_channel_id"

self_channel_response="$(json_request POST /channels "$(jq -n \
  --arg base_url "${API_BASE%/api/v1}" \
  --arg source_code "$cascade_source_code" \
  --arg source_token "$cascade_source_token" \
  '{
    provider_type:"self",
    name:"Smoke 本平台级联",
    enabled:true,
    auth_config:{
      base_url:$base_url,
      source_code:$source_code,
      source_token:$source_token
    },
    token_config:{},
    send_config:{
      api_prefix:"/api/v1",
      payload_mode:"wrapped"
    },
    rate_limit_config:{},
    concurrency_limit:2,
    timeout_ms:5000,
    retry_policy:{max_attempts:2},
    dead_letter_policy:{enabled:true}
  }')" "$admin_token")"
self_channel_id="$(jq -r '.channel.id' <<<"$self_channel_response")"
echo "created self cascade channel: $self_channel_id"

webhook_template_response="$(json_request POST /templates "$(jq -n \
  --arg source_id "$source_id" \
  '{
    name:"Smoke Webhook 模板",
    description:"端到端验收 Webhook 模板",
    source_id:$source_id,
    enabled:true
  }')" "$admin_token")"
webhook_template_id="$(jq -r '.template.id' <<<"$webhook_template_response")"

smoke_template_body='{"title":"{{ payload.title }}","content":"{{ payload.content }}","severity":"{{ payload.severity }}","bizId":"{{ payload.bizId }}"}'
webhook_template_publish_body="$(jq -n \
  --arg template_body "$smoke_template_body" \
  '{
    message_type:"json",
    target_provider_type:"webhook",
    template_body:$template_body,
    message_body_schema:{type:"object"},
    sample_payload:{title:"Smoke 消息",content:"端到端验收",severity:"info",bizId:"SMOKE-001"}
  }')"
webhook_template_version_response="$(json_request POST "/templates/$webhook_template_id/publish" "$webhook_template_publish_body" "$admin_token")"
webhook_template_version_id="$(jq -r '.version.id' <<<"$webhook_template_version_response")"
echo "published webhook template version: $webhook_template_version_id"

self_template_response="$(json_request POST /templates "$(jq -n \
  --arg source_id "$source_id" \
  '{
    name:"Smoke 级联模板",
    description:"端到端验收本平台级联模板",
    source_id:$source_id,
    enabled:true
  }')" "$admin_token")"
self_template_id="$(jq -r '.template.id' <<<"$self_template_response")"
self_template_publish_body="$(jq -n \
  --arg template_body "$smoke_template_body" \
  '{
    message_type:"json",
    target_provider_type:"self",
    template_body:$template_body,
    message_body_schema:{type:"object"},
    sample_payload:{title:"Smoke 消息",content:"端到端验收",severity:"info",bizId:"SMOKE-001"}
  }')"
self_template_version_response="$(json_request POST "/templates/$self_template_id/publish" "$self_template_publish_body" "$admin_token")"
self_template_version_id="$(jq -r '.version.id' <<<"$self_template_version_response")"
echo "published self template version: $self_template_version_id"

flow_response="$(json_request POST /route-flows "$(jq -n \
  --arg source_id "$source_id" \
  '{
    source_id:$source_id,
    name:"Smoke 路由",
    enabled:true,
    mode:"table"
  }')" "$admin_token")"
flow_id="$(jq -r '.flow.id' <<<"$flow_response")"

rules_response="$(json_request PUT "/route-flows/$flow_id/rules" "$(jq -n \
  --arg webhook_template_version_id "$webhook_template_version_id" \
  --arg webhook_channel_id "$webhook_channel_id" \
  --arg self_template_version_id "$self_template_version_id" \
  --arg self_channel_id "$self_channel_id" \
  '{
    rules:[{
      sort_order:10,
      name:"默认 Smoke 规则",
      enabled:true,
      condition_tree:{operator:"always"},
      action:{
        targets:[{
          channel_id:$webhook_channel_id,
          template_version_id:$webhook_template_version_id,
          enabled:true
        },{
          channel_id:$self_channel_id,
          template_version_id:$self_template_version_id,
          enabled:true
        }],
        recipient_strategy:{mode:"none"},
        send_dedupe_config:{},
        failure_policy:{}
      }
    }]
  }')" "$admin_token")"
rule_key="$(jq -r '.rules[0].rule_key' <<<"$rules_response")"
echo "saved route rule: $rule_key"

version_response="$(json_request POST "/route-flows/$flow_id/publish" "" "$admin_token")"
route_version_id="$(jq -r '.version.id' <<<"$version_response")"
json_request POST "/route-flows/$flow_id/versions/$route_version_id/activate" "" "$admin_token" >/dev/null
echo "activated route version: $route_version_id"

ingest_response="$(json_request POST "/ingest/$source_code" '{
  "title": "Smoke 消息",
  "content": "端到端验收",
  "severity": "info",
  "bizId": "SMOKE-001"
}' "$source_token")"
trace_id="$(jq -r '.trace_id' <<<"$ingest_response")"
echo "ingested payload trace_id: $trace_id"

for _ in $(seq 1 20); do
  message_list="$(json_request GET "/messages?trace_id=$trace_id" "" "$admin_token")"
  message_id="$(jq -r '.messages[0].id // empty' <<<"$message_list")"
  status="$(jq -r '.messages[0].status // empty' <<<"$message_list")"
  outbound_status="$(jq -r '.messages[0].outbound_status // empty' <<<"$message_list")"
  if [[ -n "$message_id" ]] && {
    [[ "$status" == "no_route" || "$status" == "plan_failed" ]] ||
    [[ "$outbound_status" == "sent" || "$outbound_status" == "failed" || "$outbound_status" == "dead_letter" ]]
  }; then
    echo "message status: $status, outbound: ${outbound_status:-unknown}"
    detail="$(json_request GET "/messages/$message_id" "" "$admin_token")"
    attempt_count="$(jq -r '.message.attempt_count // 0' <<<"$detail")"
    if [[ "$attempt_count" -lt 2 ]]; then
      echo "expected at least 2 delivery attempts, got $attempt_count" >&2
      jq '{trace_id:.message.trace_id,status:.message.status,outbound_status:.message.outbound_status,attempt_count:.message.attempt_count,timeline:.message.timeline,attempts:.message.attempts}' <<<"$detail" >&2
      exit 1
    fi
    jq '{trace_id:.message.trace_id,status:.message.status,outbound_status:.message.outbound_status,attempt_count:.message.attempt_count,timeline:.message.timeline,attempts:[.message.attempts[] | {channel_id,template_version_id,status}]}' <<<"$detail"
    exit 0
  fi
  sleep 1
done

echo "message was accepted but did not finish within 20 seconds; inspect trace_id=$trace_id" >&2
exit 2
