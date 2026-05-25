import { describe, it, expect } from 'vitest';
import { formatPhase } from '@/lib/install-progress';

// formatPhase renders the step_progress.data payload as the sticky status
// banner string shown above the log stream during a bundle install. These
// tests pin the user-visible copy for each phase variant — regressions in
// the strings are likely what an operator would notice first. See issue #42.
describe('formatPhase', () => {
  it('renders bundle_start with chart count', () => {
    expect(
      formatPhase({ phase: 'bundle_start', bundle: 'homelab-essentials', total: 4 })
    ).toBe('Starting bundle homelab-essentials — 4 charts to install…');
  });

  it('singular chart count loses the plural s', () => {
    expect(
      formatPhase({ phase: 'bundle_start', bundle: 'singleton', total: 1 })
    ).toBe('Starting bundle singleton — 1 chart to install…');
  });

  it('renders chart_install with N of M and chart identity', () => {
    expect(
      formatPhase({
        phase: 'chart_install',
        chart: 'longhorn/longhorn',
        release: 'longhorn',
        namespace: 'longhorn-system',
        index: 2,
        total: 4,
      })
    ).toBe('Installing chart 2 of 4: longhorn/longhorn (release=longhorn, ns=longhorn-system)');
  });

  it('renders rollback with failed chart name and count', () => {
    expect(
      formatPhase({ phase: 'rollback', failed_chart: 'wikijs/wiki', rollback_count: 2 })
    ).toBe('Rolling back 2 previously-installed charts after wikijs/wiki failed…');
  });

  it('rollback single chart is singular', () => {
    expect(
      formatPhase({ phase: 'rollback', failed_chart: 'loki', rollback_count: 1 })
    ).toBe('Rolling back 1 previously-installed chart after loki failed…');
  });
});
