import { useState, type ReactNode, type MouseEvent } from "react";
import {
  AlertOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
  BarChartOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  CloudServerOutlined,
  CopyOutlined,
  CheckOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  ExclamationCircleOutlined,
  HourglassOutlined,
  InboxOutlined,
  NodeIndexOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  SendOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import Button from "antd/es/button";
import Pagination from "antd/es/pagination";
import Space from "antd/es/space";
import Typography from "antd/es/typography";
import message from "antd/es/message";

import type { TagMeta } from "../utils/labels";

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
  createText = "新增",
  extra,
  className = "",
}: {
  children: ReactNode;
  onCreate?: () => void;
  onSearch?: () => void;
  onReset?: () => void;
  createText?: string;
  extra?: ReactNode;
  className?: string;
}) {
  return (
    <section
      className={`query-bar${className ? ` ${className}` : ""}`}
      aria-label="查询栏"
    >
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
  currentPage = 1,
  extra,
  fill = false,
  scrollY,
  className = "",
  onPageChange,
}: {
  title: ReactNode;
  children: ReactNode;
  total: number;
  pageSize?: number;
  currentPage?: number;
  extra?: ReactNode;
  fill?: boolean;
  scrollY?: number;
  className?: string;
  onPageChange?: (page: number, pageSize: number) => void;
}) {
  const scrollStyle = scrollY && !fill ? { maxHeight: scrollY } : undefined;
  const classNames = [
    "list-container",
    fill ? "list-container--fill" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <section className={classNames}>
      <div className="list-container__header">
        {typeof title === "string" ? (
          <Typography.Title level={4}>{title}</Typography.Title>
        ) : (
          <div className="list-container__title-node">{title}</div>
        )}
        {extra}
      </div>
      <div className="table-scroll" style={scrollStyle}>
        {children}
      </div>
      <div className="inline-pagination">
        <Typography.Text type="secondary">共 {total} 条</Typography.Text>
        <Pagination
          current={currentPage}
          pageSize={pageSize}
          total={total}
          onChange={onPageChange}
          showSizeChanger
          pageSizeOptions={[10, 20, 50]}
        />
      </div>
    </section>
  );
}

export function StatusTag({ meta }: { meta: TagMeta }) {
  const colorClass = `status-tag--${meta.color || "default"}`;
  return (
    <span className={`premium-status-tag ${colorClass}`}>
      <span className="status-dot" />
      <span className="status-label">{meta.label}</span>
    </span>
  );
}

export function CopyableIdentifier({
  value,
  code = false,
  maxWidth = 180,
}: {
  value: string;
  code?: boolean;
  maxWidth?: number;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async (e: MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      void message.success({ content: "已复制到剪贴板", duration: 1.5 });
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      const textArea = document.createElement("textarea");
      textArea.value = value;
      document.body.appendChild(textArea);
      textArea.select();
      try {
        document.execCommand("copy");
        setCopied(true);
        void message.success({ content: "已复制到剪贴板", duration: 1.5 });
        setTimeout(() => setCopied(false), 2000);
      } catch (copyErr) {
        console.error("Failed to copy text: ", copyErr);
      }
      document.body.removeChild(textArea);
    }
  };

  if (!value) return <span>-</span>;

  return (
    <span className="copyable-identifier-wrapper" style={{ maxWidth }}>
      <Typography.Text
        code={code}
        ellipsis={{
          tooltip: {
            title: value,
            color: "#ffffff",
            classNames: { root: "copyable-identifier-tooltip-overlay" },
          },
        }}
        style={{
          display: "inline-block",
          maxWidth: maxWidth - 24,
          verticalAlign: "middle",
          margin: 0,
        }}
      >
        {value}
      </Typography.Text>
      <span className="copy-action-trigger" onClick={handleCopy} title="复制">
        {copied ? (
          <CheckOutlined style={{ color: "#52c41a" }} />
        ) : (
          <CopyOutlined />
        )}
      </span>
    </span>
  );
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
  trend: "up" | "down" | "flat";
  accent: "blue" | "green" | "orange" | "red" | "purple";
  footnote?: string;
}) {
  const TrendIcon = trend === "up" ? ArrowUpOutlined : ArrowDownOutlined;
  const fallbackIcon =
    accent === "green" ? (
      <SafetyCertificateOutlined />
    ) : accent === "red" ? (
      <ExclamationCircleOutlined />
    ) : accent === "orange" ? (
      <ClockCircleOutlined />
    ) : accent === "purple" ? (
      <BarChartOutlined />
    ) : label.includes("积压") ? (
      <DatabaseOutlined />
    ) : label.includes("发送") ? (
      <SendOutlined />
    ) : (
      <CloudServerOutlined />
    );

  // Premium, visually appropriate icons based on card label
  const premiumIcon =
    label === "总接收量" ? (
      <InboxOutlined />
    ) : label === "总发送量" ? (
      <SendOutlined />
    ) : label === "成功发送量" ? (
      <CheckCircleOutlined />
    ) : label === "失败发送量" ? (
      <CloseCircleOutlined />
    ) : label.includes("成功率") ? (
      <DashboardOutlined />
    ) : label === "平均 OPS" ? (
      <ThunderboltOutlined />
    ) : label === "路由规划积压" ? (
      <NodeIndexOutlined />
    ) : label === "出站发送积压" ? (
      <SendOutlined />
    ) : label === "最老任务等待" ? (
      <ClockCircleOutlined />
    ) : label.includes("平均耗时") ? (
      <HourglassOutlined />
    ) : label === "死信数量" ? (
      <AlertOutlined />
    ) : (
      (icon ?? fallbackIcon)
    );

  // Minimalist micro SVG sparkline/ring based on card label to make the dashboard look highly premium
  const renderMicroChart = () => {
    if (label.includes("成功率") || label.includes("健康度")) {
      return (
        <div className="metric-donut-container" aria-hidden="true">
          <svg className="metric-donut" viewBox="0 0 36 36">
            <path
              className="donut-ring"
              d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
              fill="none"
              stroke="rgba(var(--accent-rgb), 0.15)"
              strokeWidth="3.2"
            />
            <path
              className="donut-segment"
              d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
              fill="none"
              stroke="currentColor"
              strokeWidth="3.2"
              strokeDasharray="96, 100"
              strokeLinecap="round"
            />
          </svg>
        </div>
      );
    }

    let dPath = "M0,25 Q15,10 30,20 T60,12 T90,25 T100,15"; // Default wave
    if (label === "总接收量") {
      dPath = "M0,25 Q20,5 40,20 T80,10 T100,18";
    } else if (label === "总发送量") {
      dPath = "M0,28 C20,25 40,5 60,12 C80,18 95,2 100,6";
    } else if (label === "成功发送量") {
      dPath = "M0,26 L20,22 L45,8 L70,10 L85,2 L100,0";
    } else if (label === "失败发送量" || label === "死信数量") {
      dPath = "M0,28 L30,28 L50,28 L70,6 L85,24 L100,28";
    } else if (label === "平均 OPS") {
      dPath =
        "M0,15 L10,10 L20,20 L30,5 L40,25 L50,8 L60,22 L70,12 L80,18 L90,6 L100,15";
    } else if (label === "最老任务等待") {
      dPath = "M0,28 L20,28 L40,28 L60,28 L80,28 L100,28";
    } else if (label.includes("积压")) {
      dPath = "M0,28 Q25,28 50,15 T100,25";
    } else if (label.includes("平均耗时")) {
      dPath = "M0,20 L15,18 L30,22 L55,10 L75,12 L100,8";
    }

    return (
      <div className="metric-sparkline-container" aria-hidden="true">
        <svg
          className="metric-sparkline"
          viewBox="0 0 100 30"
          preserveAspectRatio="none"
        >
          <path
            d={dPath}
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </div>
    );
  };

  return (
    <div className={`metric-card metric-card--${accent}`}>
      <div>
        <span className="metric-label">{label}</span>
        <strong>{value}</strong>
        <span className={`metric-delta metric-delta--${trend}`}>
          {trend === "flat" ? null : <TrendIcon />} {delta}
        </span>
        {footnote ? <span className="metric-footnote">{footnote}</span> : null}
      </div>
      <span className="metric-icon" aria-hidden="true">
        {premiumIcon}
      </span>
      {renderMicroChart()}
    </div>
  );
}

export function LineChart({
  points,
  labels,
  seriesLabel = "趋势",
  series,
}: {
  points: number[];
  labels?: string[];
  seriesLabel?: string;
  series?: Array<{
    key: string;
    label: string;
    color: string;
    points: number[];
  }>;
}) {
  const width = 720;
  const height = 260;
  const padding = { top: 24, right: 26, bottom: 40, left: 50 };
  const normalizedSeries =
    series && series.length > 0
      ? series
      : [
          {
            key: "default",
            label: seriesLabel,
            color: "#1677ff",
            points,
          },
        ];
  const allPoints = normalizedSeries.flatMap((item) => item.points);
  const primarySeries = normalizedSeries[0];
  const max = Math.max(...allPoints, 1);
  const min = Math.min(...allPoints, 0);
  const range = Math.max(max - min, 1);
  const innerWidth = width - padding.left - padding.right;
  const innerHeight = height - padding.top - padding.bottom;
  const seriesCoords = normalizedSeries.map((item) => ({
    ...item,
    coords: item.points.map((point, index) => {
      const x =
        padding.left +
        (innerWidth * index) / Math.max(item.points.length - 1, 1);
      const y =
        padding.top + innerHeight - ((point - min) / range) * innerHeight;
      return { x, y, point };
    }),
  }));
  const primaryCoords = primarySeries.points.map((point, index) => {
    const x =
      padding.left +
      (innerWidth * index) / Math.max(primarySeries.points.length - 1, 1);
    const y = padding.top + innerHeight - ((point - min) / range) * innerHeight;
    return { x, y, point };
  });
  const linePath = primaryCoords
    .map(({ x, y }, index) => `${index === 0 ? "M" : "L"} ${x} ${y}`)
    .join(" ");
  const areaPath = `${linePath} L ${padding.left + innerWidth} ${padding.top + innerHeight} L ${padding.left} ${
    padding.top + innerHeight
  } Z`;
  const yTicks = [max, min + range * 0.66, min + range * 0.33, min];
  const xLabels = labels ?? ["00:00", "06:00", "12:00", "18:00", "24:00"];
  const xAxisLabels = xLabels
    .map((label, index) => ({ label, index }))
    .filter((_, index) => shouldRenderAxisLabel(index, xLabels.length));
  const xLabelDenominator =
    labels?.length === primarySeries.points.length
      ? Math.max(primarySeries.points.length - 1, 1)
      : Math.max(xLabels.length - 1, 1);

  return (
    <div className="line-chart" aria-label={seriesLabel}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img">
        <defs>
          <linearGradient
            id={`chart-area-${seriesLabel}`}
            x1="0"
            x2="0"
            y1="0"
            y2="1"
          >
            <stop offset="0%" stopColor="#1677ff" stopOpacity="0.2" />
            <stop offset="100%" stopColor="#1677ff" stopOpacity="0.02" />
          </linearGradient>
        </defs>
        {yTicks.map((tick) => {
          const y =
            padding.top + innerHeight - ((tick - min) / range) * innerHeight;
          return (
            <g key={tick}>
              <line
                x1={padding.left}
                x2={padding.left + innerWidth}
                y1={y}
                y2={y}
                className="chart-grid"
              />
              <text
                x={padding.left - 10}
                y={y + 4}
                textAnchor="end"
                className="chart-axis-label"
              >
                {formatChartTick(tick)}
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
        {xAxisLabels.map(({ label, index }) => {
          const x = padding.left + (innerWidth * index) / xLabelDenominator;
          return (
            <text
              key={`${label}-${index}`}
              x={x}
              y={height - 14}
              textAnchor="middle"
              className="chart-axis-label"
            >
              {label}
            </text>
          );
        })}
        <path d={areaPath} fill={`url(#chart-area-${seriesLabel})`} />
        {seriesCoords.map((item) => {
          const path = item.coords
            .map(({ x, y }, index) => `${index === 0 ? "M" : "L"} ${x} ${y}`)
            .join(" ");
          return (
            <path
              key={item.key}
              d={path}
              className="chart-line"
              style={{ stroke: item.color }}
            />
          );
        })}
        {primaryCoords.map(({ x, y, point }, index) =>
          shouldRenderPointLabel(point, index, primaryCoords.length) ? (
            <g key={`${point}-${index}`}>
              <circle
                cx={x}
                cy={y}
                r="3.2"
                className="chart-point"
                style={{ fill: primarySeries.color }}
              />
              <text
                x={x}
                y={y - 10}
                dx={pointLabelOffset(index, primaryCoords.length)}
                textAnchor={pointLabelAnchor(index, primaryCoords.length)}
                className="chart-point-label"
              >
                {point}
              </text>
            </g>
          ) : null,
        )}
      </svg>
      {series && series.length > 0 ? (
        <div className="legend-row">
          {normalizedSeries.map((item) => (
            <span className="chart-legend-item" key={item.key}>
              <span
                className="chart-legend-dot"
                style={{ backgroundColor: item.color }}
              />
              <span className="chart-legend-label">{item.label}</span>
            </span>
          ))}
        </div>
      ) : null}
    </div>
  );
}

export function GroupedBarChart({
  labels,
  series,
  ariaLabel = "柱状图",
  activeLabel,
  onPointClick,
}: {
  labels: string[];
  series: Array<{
    key: string;
    label: string;
    color: string;
    points: number[];
  }>;
  ariaLabel?: string;
  activeLabel?: string;
  onPointClick?: (label: string, index: number) => void;
}) {
  const width = 720;
  const height = 260;
  const padding = { top: 24, right: 26, bottom: 42, left: 68 };
  const normalizedSeries = series.length > 0 ? series : [];
  const allPoints = normalizedSeries.flatMap((item) => item.points);
  const max = Math.max(...allPoints, 1);
  const innerWidth = width - padding.left - padding.right;
  const innerHeight = height - padding.top - padding.bottom;
  const bucketCount = Math.max(
    labels.length,
    ...normalizedSeries.map((item) => item.points.length),
    1,
  );
  const bucketWidth = innerWidth / bucketCount;
  const gap = Math.min(16, bucketWidth * 0.28);
  const groupWidth = Math.max(bucketWidth - gap, 6);
  const seriesGap = 3;
  const rawBarWidth = Math.max(
    groupWidth / Math.max(normalizedSeries.length, 1) - seriesGap,
    3,
  );
  const barWidth = Math.min(rawBarWidth, 42);
  const renderedGroupWidth =
    normalizedSeries.length * barWidth +
    Math.max(normalizedSeries.length - 1, 0) * seriesGap;
  const yTicks = [max, max * 0.66, max * 0.33, 0];
  const xAxisLabels = labels
    .map((label, index) => ({ label, index }))
    .filter((_, index) => shouldRenderAxisLabel(index, labels.length));

  return (
    <div className="bar-chart" aria-label={ariaLabel}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img">
        {yTicks.map((tick) => {
          const y = padding.top + innerHeight - (tick / max) * innerHeight;
          return (
            <g key={tick}>
              <line
                x1={padding.left}
                x2={padding.left + innerWidth}
                y1={y}
                y2={y}
                className="chart-grid"
              />
              <text
                x={padding.left - 10}
                y={y + 4}
                textAnchor="end"
                className="chart-axis-label"
              >
                {formatChartTick(tick)}
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
        {xAxisLabels.map(({ label, index }) => {
          const x = padding.left + bucketWidth * index + bucketWidth / 2;
          return (
            <text
              key={`${label}-${index}`}
              x={x}
              y={height - 14}
              textAnchor="middle"
              className="chart-axis-label"
            >
              {label}
            </text>
          );
        })}
        {normalizedSeries.flatMap((item, seriesIndex) =>
          item.points.map((point, index) => {
            const value = Math.max(0, Number.isFinite(point) ? point : 0);
            const barHeight = (value / max) * innerHeight;
            const x =
              padding.left +
              bucketWidth * index +
              (bucketWidth - renderedGroupWidth) / 2 +
              seriesIndex * (barWidth + seriesGap);
            const y = padding.top + innerHeight - barHeight;
            const label = labels[index] ?? String(index + 1);
            const active = activeLabel === label;
            return (
              <rect
                key={`${item.key}-${index}`}
                x={x}
                y={y}
                width={barWidth}
                height={Math.max(barHeight, value > 0 ? 2 : 0)}
                rx="3"
                className={`chart-bar${active ? " chart-bar--active" : ""}${onPointClick ? " chart-bar--clickable" : ""}`}
                style={{ fill: item.color }}
                onClick={() => onPointClick?.(label, index)}
              >
                <title>
                  {`并发 ${label} / ${item.label}: ${formatChartTick(value)}`}
                </title>
              </rect>
            );
          }),
        )}
      </svg>
      <div className="chart-inline-legend">
        {normalizedSeries.map((item) => (
          <span className="chart-legend-item" key={item.key}>
            <span
              className="chart-legend-dot"
              style={{ backgroundColor: item.color }}
            />
            <span className="chart-legend-label">{item.label}</span>
          </span>
        ))}
      </div>
    </div>
  );
}

function shouldRenderAxisLabel(index: number, total: number): boolean {
  if (total <= 6) {
    return true;
  }
  if (index === 0 || index === total - 1) {
    return true;
  }
  const interval = Math.ceil((total - 1) / 4);
  return index % interval === 0;
}

function shouldRenderPointLabel(
  point: number,
  index: number,
  total: number,
): boolean {
  if (point <= 0) {
    return false;
  }
  return index % 4 === 0 || index === total - 1;
}

function pointLabelAnchor(
  index: number,
  total: number,
): "start" | "middle" | "end" {
  if (index === 0) {
    return "start";
  }
  if (index === total - 1) {
    return "end";
  }
  return "middle";
}

function pointLabelOffset(index: number, total: number): number {
  if (index === 0) {
    return 6;
  }
  if (index === total - 1) {
    return -6;
  }
  return 0;
}

function formatChartTick(value: number): string {
  if (
    value === undefined ||
    value === null ||
    Number.isNaN(value) ||
    typeof value !== "number"
  ) {
    return "0";
  }
  if (Number.isInteger(value)) {
    return value.toLocaleString("zh-CN");
  }
  return value.toLocaleString("zh-CN", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 1,
  });
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
