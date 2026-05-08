import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { Button, Pagination, Space, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';

import { formatRefreshTime, type TagMeta } from '../utils/labels';

export type PageFrameProps = {
  title: string;
  description?: string;
  children: ReactNode;
  extra?: ReactNode;
  lastUpdated: Date;
  onRefresh: () => void;
  showPolling?: boolean;
};

export function PageFrame({
  title,
  description,
  children,
  extra,
  lastUpdated,
  onRefresh,
  showPolling = true,
}: PageFrameProps) {
  return (
    <main className="page-frame">
      <div className="page-heading">
        <div>
          <Typography.Title level={2} className="page-title">
            {title}
          </Typography.Title>
          {description ? (
            <Typography.Text type="secondary">{description}</Typography.Text>
          ) : null}
        </div>
        <Space wrap className="page-actions">
          {showPolling ? <PollingStatus lastUpdated={lastUpdated} /> : null}
          {extra}
          <Button icon={<ReloadOutlined />} onClick={onRefresh}>
            手动刷新
          </Button>
        </Space>
      </div>
      {children}
    </main>
  );
}

export function PollingStatus({ lastUpdated }: { lastUpdated: Date }) {
  return (
    <Space size={8} className="polling-status">
      <Tag color="success">5 秒轮询</Tag>
      <Typography.Text type="secondary">
        最近刷新：{formatRefreshTime(lastUpdated)}
      </Typography.Text>
    </Space>
  );
}

export function QueryBar({
  children,
  onCreate,
  createText = '新增',
  extra,
}: {
  children: ReactNode;
  onCreate?: () => void;
  createText?: string;
  extra?: ReactNode;
}) {
  return (
    <section className="query-bar" aria-label="查询栏">
      <div className="query-fields">{children}</div>
      <Space wrap className="query-actions">
        <Button>重置</Button>
        <Button type="primary">查询</Button>
        {extra}
        {onCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            {createText}
          </Button>
        ) : null}
      </Space>
    </section>
  );
}

export function ListContainer({
  title,
  children,
  total,
  pageSize = 20,
  extra,
}: {
  title: string;
  children: ReactNode;
  total: number;
  pageSize?: number;
  extra?: ReactNode;
}) {
  return (
    <section className="list-container">
      <div className="list-container__header">
        <Typography.Title level={4}>{title}</Typography.Title>
        {extra}
      </div>
      <div className="table-scroll">{children}</div>
      <div className="inline-pagination">
        <Typography.Text type="secondary">共 {total} 条</Typography.Text>
        <Pagination
          current={1}
          pageSize={pageSize}
          total={total}
          onChange={() => undefined}
          showSizeChanger
          pageSizeOptions={[10, 20, 50]}
        />
      </div>
    </section>
  );
}

export function StatusTag({ meta }: { meta: TagMeta }) {
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

export function MetricCard({
  label,
  value,
  delta,
  trend,
  accent,
  footnote,
}: {
  label: string;
  value: string;
  delta: string;
  trend: 'up' | 'down' | 'flat';
  accent: 'blue' | 'green' | 'orange' | 'red' | 'purple';
  footnote?: string;
}) {
  const TrendIcon = trend === 'up' ? ArrowUpOutlined : ArrowDownOutlined;
  return (
    <div className={`metric-card metric-card--${accent}`}>
      <div>
        <span className="metric-label">{label}</span>
        <strong>{value}</strong>
        <span className={`metric-delta metric-delta--${trend}`}>
          {trend === 'flat' ? null : <TrendIcon />} {delta}
        </span>
        {footnote ? <span className="metric-footnote">{footnote}</span> : null}
      </div>
      <span className="metric-signal" />
    </div>
  );
}

export function MiniTrend({ points }: { points: number[] }) {
  return (
    <div className="mini-trend" aria-label="趋势图">
      {points.map((point, index) => (
        <span
          // Index is stable because demo trend order is immutable and not user-edited.
          key={`${point}-${index}`}
          style={{ height: `${point}%` }}
        />
      ))}
    </div>
  );
}

export function FieldLabel({ label }: { label: string }) {
  return <span className="field-label">{label}</span>;
}
