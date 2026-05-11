import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  BarChartOutlined,
  ClockCircleOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  ExclamationCircleOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  SendOutlined,
} from '@ant-design/icons';
import Button from 'antd/es/button';
import Pagination from 'antd/es/pagination';
import Space from 'antd/es/space';
import Tag from 'antd/es/tag';
import Typography from 'antd/es/typography';
import type { ReactNode } from 'react';

import type { TagMeta } from '../utils/labels';

export type PageFrameProps = {
  title: string;
  description?: string;
  children: ReactNode;
  extra?: ReactNode;
  lastUpdated?: Date;
  onRefresh?: () => void;
  showPolling?: boolean;
};

export function PageFrame({
  title,
  description,
  children,
  extra,
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
        {extra ? (
          <Space wrap className="page-actions">
            {extra}
          </Space>
        ) : null}
      </div>
      {children}
    </main>
  );
}

export function QueryBar({
  children,
  onCreate,
  onSearch,
  onReset,
  createText = '新增',
  extra,
}: {
  children: ReactNode;
  onCreate?: () => void;
  onSearch?: () => void;
  onReset?: () => void;
  createText?: string;
  extra?: ReactNode;
}) {
  return (
    <section className="query-bar" aria-label="查询栏">
      <div className="query-fields">{children}</div>
      <Space wrap className="query-actions">
        <Button onClick={onReset}>重置</Button>
        <Button type="primary" onClick={onSearch}>
          查询
        </Button>
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
  fill = false,
  scrollY,
}: {
  title: string;
  children: ReactNode;
  total: number;
  pageSize?: number;
  extra?: ReactNode;
  fill?: boolean;
  scrollY?: number;
}) {
  const scrollStyle = scrollY && !fill ? { maxHeight: scrollY } : undefined;

  return (
    <section className={`list-container${fill ? ' list-container--fill' : ''}`}>
      <div className="list-container__header">
        <Typography.Title level={4}>{title}</Typography.Title>
        {extra}
      </div>
      <div className="table-scroll" style={scrollStyle}>
        {children}
      </div>
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
  icon,
  label,
  value,
  delta,
  trend,
  accent,
  footnote,
}: {
  icon?: ReactNode;
  label: string;
  value: string;
  delta: string;
  trend: 'up' | 'down' | 'flat';
  accent: 'blue' | 'green' | 'orange' | 'red' | 'purple';
  footnote?: string;
}) {
  const TrendIcon = trend === 'up' ? ArrowUpOutlined : ArrowDownOutlined;
  const fallbackIcon =
    accent === 'green' ? (
      <SafetyCertificateOutlined />
    ) : accent === 'red' ? (
      <ExclamationCircleOutlined />
    ) : accent === 'orange' ? (
      <ClockCircleOutlined />
    ) : accent === 'purple' ? (
      <BarChartOutlined />
    ) : label.includes('积压') ? (
      <DatabaseOutlined />
    ) : label.includes('发送') ? (
      <SendOutlined />
    ) : (
      <CloudServerOutlined />
    );
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
      <span className="metric-icon" aria-hidden="true">
        {icon ?? fallbackIcon}
      </span>
    </div>
  );
}

export function LineChart({
  points,
  labels,
  seriesLabel = '趋势',
}: {
  points: number[];
  labels?: string[];
  seriesLabel?: string;
}) {
  const width = 720;
  const height = 260;
  const padding = { top: 24, right: 26, bottom: 40, left: 50 };
  const max = Math.max(...points, 1);
  const min = Math.min(...points, 0);
  const range = Math.max(max - min, 1);
  const innerWidth = width - padding.left - padding.right;
  const innerHeight = height - padding.top - padding.bottom;
  const coords = points.map((point, index) => {
    const x = padding.left + (innerWidth * index) / Math.max(points.length - 1, 1);
    const y = padding.top + innerHeight - ((point - min) / range) * innerHeight;
    return { x, y, point };
  });
  const linePath = coords.map(({ x, y }, index) => `${index === 0 ? 'M' : 'L'} ${x} ${y}`).join(' ');
  const areaPath = `${linePath} L ${padding.left + innerWidth} ${padding.top + innerHeight} L ${padding.left} ${
    padding.top + innerHeight
  } Z`;
  const yTicks = [max, min + range * 0.66, min + range * 0.33, min];
  const xLabels = labels ?? ['00:00', '06:00', '12:00', '18:00', '24:00'];

  return (
    <div className="line-chart" aria-label={seriesLabel}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img">
        <defs>
          <linearGradient id={`chart-area-${seriesLabel}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="#1677ff" stopOpacity="0.2" />
            <stop offset="100%" stopColor="#1677ff" stopOpacity="0.02" />
          </linearGradient>
        </defs>
        {yTicks.map((tick) => {
          const y = padding.top + innerHeight - ((tick - min) / range) * innerHeight;
          return (
            <g key={tick}>
              <line x1={padding.left} x2={padding.left + innerWidth} y1={y} y2={y} className="chart-grid" />
              <text x={padding.left - 10} y={y + 4} textAnchor="end" className="chart-axis-label">
                {Math.round(tick).toLocaleString('zh-CN')}
              </text>
            </g>
          );
        })}
        <line
          x1={padding.left}
          x2={padding.left}
          y1={padding.top}
          y2={padding.top + innerHeight}
          className="chart-axis"
        />
        <line
          x1={padding.left}
          x2={padding.left + innerWidth}
          y1={padding.top + innerHeight}
          y2={padding.top + innerHeight}
          className="chart-axis"
        />
        {xLabels.map((label, index) => {
          const x = padding.left + (innerWidth * index) / Math.max(xLabels.length - 1, 1);
          return (
            <text key={label} x={x} y={height - 14} textAnchor="middle" className="chart-axis-label">
              {label}
            </text>
          );
        })}
        <path d={areaPath} fill={`url(#chart-area-${seriesLabel})`} />
        <path d={linePath} className="chart-line" />
        {coords.map(({ x, y, point }, index) =>
          index % 4 === 0 || index === coords.length - 1 ? (
            <g key={`${point}-${index}`}>
              <circle cx={x} cy={y} r="4" className="chart-point" />
              <text x={x} y={y - 10} textAnchor="middle" className="chart-point-label">
                {point}
              </text>
            </g>
          ) : null,
        )}
      </svg>
    </div>
  );
}

export function MiniTrend({ points }: { points: number[] }) {
  return (
    <div className="mini-trend" aria-label="趋势图">
      {points.map((point, index) => (
        <span
          // Index is stable because trend points are rendered in backend time-bucket order.
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
