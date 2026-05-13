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
});
