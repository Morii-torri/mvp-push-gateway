import Space from 'antd/es/space';
import Typography from 'antd/es/typography';

import type { DeliveryAttemptApiRecord, JSONValue } from '../../api/console';
import { DetailDotStatus, DetailMetaList } from '../../components/ConsolePrimitives';
import { getOutboundStatusMeta, getProviderTypeLabel, type ProviderType } from '../../utils/labels';
import { isRecord, normalizeOutboundStatus, stringField, stringifyJSON } from './shared';

export function MessageLogAttemptBlocks({ attempts }: { attempts: DeliveryAttemptApiRecord[] }) {
  if (!attempts.length) {
    return (
      <div className="message-log-empty-state">
        <Typography.Text type="secondary">暂无出站投递尝试</Typography.Text>
      </div>
    );
  }

  return (
    <Space direction="vertical" size={14} className="full-width message-log-attempts">
      {attempts.map((attempt, index) => {
        const targetContext = deliveryAttemptTargetContext(attempt);
        const context: Record<string, JSONValue> = isRecord(targetContext) ? targetContext : {};
        const providerType = attempt.provider_type || stringField(context.provider_type) || '-';
        const templateVersionID = attempt.template_version_id || stringField(context.template_version_id) || '-';
        const messageType = stringField(context.message_type);
        const status = normalizeOutboundStatus(attempt.status);
        const title = `发送目标 ${index + 1}`;
        const channelLabel = attempt.channel_name || attempt.channel_id || stringField(context.channel_name) || '-';
        const renderedMessage = deliveryAttemptRenderedMessage(attempt);
        const resolvedRecipients = deliveryAttemptResolvedRecipients(attempt);
        const finalRequest = deliveryAttemptFinalRequest(attempt);
        const upstreamResponse = deliveryAttemptUpstreamResponse(attempt);

        return (
          <section className="message-log-attempt" key={attempt.id || `${attempt.channel_id}-${index}`}>
            <div className="panel-heading">
              <Typography.Title level={5}>{title}</Typography.Title>
              <DetailDotStatus meta={getOutboundStatusMeta(status)} />
            </div>
            <DetailMetaList
              className="message-log-attempt-meta"
              items={[
                { label: '推送渠道实例', value: channelLabel },
                {
                  label: '平台类型',
                  value: providerType === '-' ? '-' : getProviderTypeLabel(providerType as ProviderType),
                },
                { label: '模板版本', value: templateVersionID, mono: true },
                { label: '消息类型', value: messageType || '-' },
                { label: '状态', value: <DetailDotStatus meta={getOutboundStatusMeta(status)} /> },
                {
                  label: '耗时',
                  value: typeof attempt.duration_ms === 'number' ? `${attempt.duration_ms} ms` : '-',
                },
                {
                  label: '错误',
                  value: [attempt.error_code, attempt.error_message].filter(Boolean).join(' / ') || '-',
                },
              ]}
            />
            <div className="message-log-attempt-grid">
              <section>
                <Typography.Text strong>渲染后消息</Typography.Text>
                <pre className="code-block">{stringifyJSON(renderedMessage, '-')}</pre>
              </section>
              <section>
                <Typography.Text strong>接收人解析结果</Typography.Text>
                <pre className="code-block">{stringifyJSON(resolvedRecipients, '[]')}</pre>
              </section>
              <section>
                <Typography.Text strong>最终请求</Typography.Text>
                <pre className="code-block">{stringifyJSON(finalRequest, '-')}</pre>
              </section>
              <section>
                <Typography.Text strong>上游响应</Typography.Text>
                <pre className="code-block">{stringifyJSON(upstreamResponse, '-')}</pre>
              </section>
            </div>
            <Typography.Text strong>原始快照</Typography.Text>
            <div className="message-log-snapshot-grid">
              <section>
                <Typography.Text type="secondary">Request Snapshot</Typography.Text>
                <pre className="code-block">{stringifyJSON(attempt.request_snapshot, '-')}</pre>
              </section>
              <section>
                <Typography.Text type="secondary">Response Snapshot</Typography.Text>
                <pre className="code-block">{stringifyJSON(attempt.response_snapshot, '-')}</pre>
              </section>
            </div>
          </section>
        );
      })}
    </Space>
  );
}

function deliveryAttemptTargetContext(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.target_context,
    snapshotJSONField(attempt.request_snapshot, 'target_context'),
    {
      channel_id: attempt.channel_id,
      channel_name: attempt.channel_name,
      provider_type: attempt.provider_type,
      template_version_id: attempt.template_version_id,
    },
  );
}

function deliveryAttemptRenderedMessage(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.rendered_message,
    snapshotJSONField(attempt.request_snapshot, 'rendered_message'),
    nestedSnapshotJSONField(attempt.request_snapshot, 'send', 'body'),
    {},
  );
}

function deliveryAttemptResolvedRecipients(attempt: DeliveryAttemptApiRecord): JSONValue {
  return normalizeRecipientValue(
    firstJSONValue(
      attempt.resolved_recipients,
      snapshotJSONField(attempt.request_snapshot, 'resolved_recipients'),
      nestedSnapshotJSONField(attempt.request_snapshot, 'send', 'recipient'),
      snapshotJSONField(attempt.recipient_snapshot, 'recipient'),
      [],
    ),
  );
}

function deliveryAttemptFinalRequest(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.final_request,
    snapshotJSONField(attempt.request_snapshot, 'final_request'),
    snapshotJSONField(attempt.request_snapshot, 'send'),
    {},
  );
}

function deliveryAttemptUpstreamResponse(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.upstream_response,
    snapshotJSONField(attempt.response_snapshot, 'upstream_response'),
    snapshotJSONField(attempt.response_snapshot, 'send'),
    {},
  );
}

function snapshotJSONField(snapshot: JSONValue | undefined, key: string): JSONValue | undefined {
  return isRecord(snapshot) ? snapshot[key] : undefined;
}

function nestedSnapshotJSONField(snapshot: JSONValue | undefined, parent: string, key: string): JSONValue | undefined {
  const parentValue = snapshotJSONField(snapshot, parent);
  return isRecord(parentValue) ? parentValue[key] : undefined;
}

function firstJSONValue(...values: Array<JSONValue | undefined>): JSONValue {
  const found = values.find((value) => value !== undefined && value !== null && value !== '');
  return found === undefined ? null : found;
}

function normalizeRecipientValue(value: JSONValue): JSONValue {
  if (value === null || value === '') {
    return [];
  }
  return Array.isArray(value) ? value : [value];
}
