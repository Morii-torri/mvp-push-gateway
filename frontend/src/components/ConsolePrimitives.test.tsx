import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { LineChart } from './ConsolePrimitives';

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

  it('does not render zero point labels on the baseline', () => {
    const markup = renderToStaticMarkup(
      <LineChart
        points={[0, 0, 0, 0]}
        labels={['16:10', '16:11', '16:12', '16:13']}
        seriesLabel="消息发送趋势"
      />,
    );

    expect(markup).not.toContain('chart-point-label');
  });

  it('keeps edge point labels inside the plot area', () => {
    const markup = renderToStaticMarkup(
      <LineChart
        points={[3, 1, 2, 4]}
        labels={['16:10', '16:11', '16:12', '16:13']}
        seriesLabel="消息发送趋势"
      />,
    );

    expect(markup).toContain('text-anchor="start"');
    expect(markup).toContain('dx="6"');
    expect(markup).toContain('text-anchor="end"');
    expect(markup).toContain('dx="-6"');
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
