import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import {
  DetailDotStatus,
  DetailMetaList,
  GroupedBarChart,
  LineChart,
  MixedLineBarChart,
  QueueTrendChart,
} from './ConsolePrimitives';

describe('LineChart', () => {
  it('renders supplied x-axis labels instead of fixed 24 hour labels', () => {
    const markup = renderToStaticMarkup(
      <LineChart
        points={[1, 2, 3]}
        labels={['16:45', '16:50', '16:55']}
        seriesLabel="消息发送趋势"
      />,
    );

    expect(markup).toContain('16:45');
    expect(markup).toContain('16:55');
    expect(markup).not.toContain('24:00');
  });

  it('renders hover capture zones instead of default point labels', () => {
    const markup = renderToStaticMarkup(
      <LineChart
        points={[0, 2, 4, 1]}
        labels={['16:10', '16:11', '16:12', '16:13']}
        seriesLabel="消息发送趋势"
      />,
    );

    expect(markup).toContain('chart-hover-targets');
    expect(markup).toContain('data-chart-index="0"');
    expect(markup).not.toContain('chart-point-label');
    expect(markup).not.toContain('class="chart-point"');
  });

  it('renders supplied multi-series lines and labels', () => {
    const markup = renderToStaticMarkup(
      <LineChart
        points={[1, 2]}
        labels={['16:10', '16:15']}
        series={[
          { key: 'sent', label: '发送量', points: [1, 2], color: '#1677ff' },
          { key: 'qps', label: 'QPS', points: [0.1, 0.2], color: '#7c3aed' },
        ]}
        seriesLabel="消息发送趋势"
      />,
    );

    expect(markup).toContain('发送量');
    expect(markup).toContain('QPS');
    expect(markup).toContain('chart-legend-item');
    expect(markup).toContain('chart-legend-dot');
    expect(markup).toContain('chart-legend-label');
    expect(markup).not.toContain('ant-tag');
    expect(markup.match(/chart-line/g)?.length).toBeGreaterThanOrEqual(2);
  });
});

describe('MixedLineBarChart', () => {
  it('renders hover capture zones for qps and latency buckets', () => {
    const markup = renderToStaticMarkup(
      <MixedLineBarChart
        labels={['16:10', '16:15']}
        bars={{ label: '平均耗时', color: '#94a3b8', points: [12, 20] }}
        line={{ label: 'QPS', color: '#1677ff', points: [1.2, 2.4] }}
        ariaLabel="QPS 耗时趋势"
      />,
    );

    expect(markup).toContain('chart-hover-targets');
    expect(markup).toContain('data-chart-index="1"');
    expect(markup).not.toContain('class="chart-point"');
  });

  it('renders bars as top-rounded paths instead of rounded rectangles', () => {
    const markup = renderToStaticMarkup(
      <MixedLineBarChart
        labels={['16:10', '16:15']}
        bars={{ label: '平均耗时', color: '#94a3b8', points: [12, 20] }}
        line={{ label: 'QPS', color: '#1677ff', points: [1.2, 2.4] }}
        ariaLabel="QPS 耗时趋势"
      />,
    );

    expect(markup).toContain('class="chart-bar"');
    expect(markup).toContain('<path');
    expect(markup).not.toContain('rx=');
  });
});

describe('GroupedBarChart', () => {
  it('renders slim top-rounded bars and hover capture zones', () => {
    const markup = renderToStaticMarkup(
      <GroupedBarChart
        labels={['100', '200']}
        activeLabel="100"
        series={[
          { key: 'dispatch', label: '出站 QPS', color: '#1677ff', points: [10, 12] },
          { key: 'accepted', label: '接收 QPS', color: '#12b76a', points: [9, 11] },
        ]}
      />,
    );

    expect(markup).toContain('chart-hover-targets');
    expect(markup).toContain('data-chart-index="1"');
    expect(markup).toContain('chart-bar chart-bar--active');
    expect(markup).toContain('<path');
    expect(markup).not.toContain('rx=');
  });
});

describe('QueueTrendChart', () => {
  it('renders queue throughput as lines and dead letters as compact bars', () => {
    const markup = renderToStaticMarkup(
      <QueueTrendChart
        labels={['16:10', '16:15', '16:20']}
        series={[
          { key: 'route_plan', label: '路由规划处理量', color: '#1677ff', points: [10, 12, 8] },
          { key: 'send_message', label: '出站发送处理量', color: '#22c55e', points: [9, 11, 7] },
          { key: 'dead_letters', label: '死信数量', color: '#ef4444', points: [0, 1, 0] },
        ]}
        ariaLabel="队列处理趋势"
      />,
    );

    expect(markup).toContain('queue-trend-chart');
    expect(markup.match(/chart-line/g)?.length).toBeGreaterThanOrEqual(2);
    expect(markup).toContain('queue-trend-bar');
    expect(markup).toContain('chart-hover-targets');
  });
});

describe('detail primitives', () => {
  it('renders dot status and preserves zero values in metadata lists', () => {
    const markup = renderToStaticMarkup(
      <>
        <DetailDotStatus meta={{ label: '启用', color: 'success' }} />
        <DetailMetaList
          items={[
            { label: '并发', value: 0 },
            { label: '备注', value: '' },
          ]}
        />
      </>,
    );

    expect(markup).toContain('detail-dot-status--success');
    expect(markup).toContain('启用');
    expect(markup).toContain('并发');
    expect(markup).toContain('>0</dd>');
    expect(markup).toContain('备注');
    expect(markup).toContain('>-</dd>');
  });
});
