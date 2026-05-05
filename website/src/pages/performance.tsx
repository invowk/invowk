import type {ReactNode} from 'react';
import {useEffect, useMemo, useState} from 'react';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './performance.module.css';

type WindowKey = 'last_3_months' | 'last_1_year' | 'all';
type Audience = 'users' | 'developers';
type MetricFamily = 'time' | 'memory' | 'allocations';
type ValueMode = 'absolute' | 'indexed';

type HistoryPoint = {
  record_key: string;
  release_tag: string;
  release_version: string;
  generated_at: string;
  source_kind: string;
  audience: Audience;
  metric_id: string;
  benchmark_id?: string;
  label: string;
  unit: string;
  value: number;
  lower_is_better: boolean;
  confidence: string;
  environment: {
    go_version?: string;
    cpu_model?: string;
    os?: string;
    arch?: string;
    runner?: string;
  };
};

type HistoryWindow = {
  since: string;
  record_count: number;
  points: HistoryPoint[];
};

type BenchmarkHistory = {
  schema_version: string;
  generated_at: string;
  warnings: string[];
  windows: Record<WindowKey, HistoryWindow>;
  records: unknown[];
  metrics: HistoryPoint[];
};

const windowLabels: Record<WindowKey, string> = {
  last_3_months: 'Last 3 months',
  last_1_year: 'Last 1 year',
  all: 'All history',
};

const audienceLabels: Record<Audience, string> = {
  users: 'Users',
  developers: 'Developers',
};

const familyLabels: Record<MetricFamily, string> = {
  time: 'Time',
  memory: 'Memory',
  allocations: 'Allocations',
};

const valueLabels: Record<ValueMode, string> = {
  absolute: 'Absolute',
  indexed: 'Indexed',
};

function unitMatchesFamily(unit: string, family: MetricFamily): boolean {
  if (family === 'time') {
    return unit === 'ms' || unit === 'ns/op';
  }
  if (family === 'memory') {
    return unit === 'B/op';
  }
  return unit === 'allocs/op';
}

function formatValue(value: number, unit: string, mode: ValueMode): string {
  if (mode === 'indexed') {
    return `${value.toFixed(1)}`;
  }
  if (unit === 'ms') {
    return `${value.toFixed(2)} ms`;
  }
  if (unit === 'ns/op') {
    return `${value.toFixed(2)} ns/op`;
  }
  if (unit === 'B/op') {
    return `${value.toFixed(2)} B/op`;
  }
  return `${value.toFixed(2)} ${unit}`;
}

function indexedPoints(points: HistoryPoint[], mode: ValueMode): HistoryPoint[] {
  if (mode === 'absolute') {
    return points;
  }
  const firstValues = new Map<string, number>();
  return points.map((point) => {
    if (!firstValues.has(point.metric_id)) {
      firstValues.set(point.metric_id, point.value);
    }
    const first = firstValues.get(point.metric_id) || point.value;
    return {
      ...point,
      value: first === 0 ? 100 : (point.value / first) * 100,
      unit: 'index',
    };
  });
}

function latestByMetric(points: HistoryPoint[]): HistoryPoint[] {
  const latest = new Map<string, HistoryPoint>();
  for (const point of points) {
    latest.set(point.metric_id, point);
  }
  return [...latest.values()].sort((a, b) => a.label.localeCompare(b.label));
}

function groupByMetric(points: HistoryPoint[]): Map<string, HistoryPoint[]> {
  const groups = new Map<string, HistoryPoint[]>();
  for (const point of points) {
    const group = groups.get(point.metric_id) || [];
    group.push(point);
    groups.set(point.metric_id, group);
  }
  for (const group of groups.values()) {
    group.sort((a, b) => Date.parse(a.generated_at) - Date.parse(b.generated_at));
  }
  return groups;
}

function TrendChart({points, valueMode}: {points: HistoryPoint[]; valueMode: ValueMode}) {
  const groups = groupByMetric(points);
  const values = points.map((point) => point.value);
  const min = Math.min(...values, 0);
  const max = Math.max(...values, 1);
  const timestamps = points.map((point) => Date.parse(point.generated_at));
  const minTime = Math.min(...timestamps, Date.now());
  const maxTime = Math.max(...timestamps, minTime + 1);
  const width = 900;
  const height = 340;
  const pad = {left: 64, right: 24, top: 24, bottom: 56};
  const chartWidth = width - pad.left - pad.right;
  const chartHeight = height - pad.top - pad.bottom;
  const palette = ['#4f46e5', '#0f766e', '#b45309', '#be123c', '#2563eb', '#7c3aed'];

  if (points.length === 0) {
    return (
      <div className={styles.emptyState}>
        No benchmark history is available for this view yet.
      </div>
    );
  }

  const x = (date: string) => pad.left + ((Date.parse(date) - minTime) / (maxTime - minTime || 1)) * chartWidth;
  const y = (value: number) => pad.top + chartHeight - ((value - min) / (max - min || 1)) * chartHeight;

  return (
    <svg className={styles.chart} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Benchmark performance trend chart">
      <line x1={pad.left} y1={pad.top} x2={pad.left} y2={pad.top + chartHeight} className={styles.axis} />
      <line x1={pad.left} y1={pad.top + chartHeight} x2={pad.left + chartWidth} y2={pad.top + chartHeight} className={styles.axis} />
      <text x={pad.left} y={height - 16} className={styles.axisLabel}>older</text>
      <text x={pad.left + chartWidth - 38} y={height - 16} className={styles.axisLabel}>newer</text>
      <text x={12} y={pad.top + 12} className={styles.axisLabel}>{valueMode === 'indexed' ? 'index' : 'value'}</text>
      {[...groups.entries()].map(([metricId, group], index) => {
        const color = palette[index % palette.length];
        const pointsAttr = group.map((point) => `${x(point.generated_at)},${y(point.value)}`).join(' ');
        return (
          <g key={metricId}>
            <polyline points={pointsAttr} fill="none" stroke={color} strokeWidth="3" />
            {group.map((point) => (
              <circle key={`${point.record_key}-${metricId}`} cx={x(point.generated_at)} cy={y(point.value)} r="4" fill={color}>
                <title>{`${point.label}: ${formatValue(point.value, point.unit, valueMode)} (${point.release_tag || point.release_version})`}</title>
              </circle>
            ))}
          </g>
        );
      })}
    </svg>
  );
}

function Segment<T extends string>({
  label,
  value,
  values,
  labels,
  onChange,
}: {
  label: string;
  value: T;
  values: T[];
  labels: Record<T, string>;
  onChange: (value: T) => void;
}) {
  return (
    <div className={styles.segmentGroup} aria-label={label}>
      {values.map((item) => (
        <button
          key={item}
          type="button"
          className={item === value ? styles.segmentActive : styles.segment}
          aria-pressed={item === value}
          onClick={() => onChange(item)}>
          {labels[item]}
        </button>
      ))}
    </div>
  );
}

export default function Performance(): ReactNode {
  const historyUrl = useBaseUrl('/benchmarks/history.json');
  const [history, setHistory] = useState<BenchmarkHistory | null>(null);
  const [error, setError] = useState('');
  const [windowKey, setWindowKey] = useState<WindowKey>('last_3_months');
  const [audience, setAudience] = useState<Audience>('users');
  const [metricFamily, setMetricFamily] = useState<MetricFamily>('time');
  const [valueMode, setValueMode] = useState<ValueMode>('indexed');

  useEffect(() => {
    let cancelled = false;
    fetch(historyUrl)
      .then((response) => {
        if (!response.ok) {
          throw new Error(`benchmark history request failed: ${response.status}`);
        }
        return response.json() as Promise<BenchmarkHistory>;
      })
      .then((data) => {
        if (!cancelled) {
          setHistory(data);
        }
      })
      .catch((cause: Error) => {
        if (!cancelled) {
          setError(cause.message);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [historyUrl]);

  const visiblePoints = useMemo(() => {
    const windowPoints = history?.windows?.[windowKey]?.points || [];
    const filtered = windowPoints
      .filter((point) => point.audience === audience)
      .filter((point) => unitMatchesFamily(point.unit, metricFamily));
    return indexedPoints(filtered, valueMode);
  }, [audience, history, metricFamily, valueMode, windowKey]);

  const latestRows = latestByMetric(visiblePoints);
  const reducedConfidence = visiblePoints.filter((point) => point.confidence !== 'normal' || point.source_kind !== 'json');
  const generatedAt = history?.generated_at ? new Date(history.generated_at).toLocaleDateString() : '';

  return (
    <Layout title="Performance" description="Benchmark history and release performance trends for Invowk.">
      <main className={styles.page}>
        <section className={styles.header}>
          <Heading as="h1">Performance</Heading>
          <p>
            Release benchmark history with the recent windows up front and enough detail for maintainers to inspect changes over time.
          </p>
          <div className={styles.meta}>
            <span>{history ? `${history.records.length} benchmark records` : 'Loading benchmark history'}</span>
            {generatedAt && <span>Updated {generatedAt}</span>}
          </div>
        </section>

        <section className={styles.controls}>
          <Segment label="Time window" value={windowKey} values={['last_3_months', 'last_1_year', 'all']} labels={windowLabels} onChange={setWindowKey} />
          <Segment label="Audience" value={audience} values={['users', 'developers']} labels={audienceLabels} onChange={setAudience} />
          <Segment label="Metric" value={metricFamily} values={['time', 'memory', 'allocations']} labels={familyLabels} onChange={setMetricFamily} />
          <Segment label="Value mode" value={valueMode} values={['indexed', 'absolute']} labels={valueLabels} onChange={setValueMode} />
        </section>

        <section className={styles.panel}>
          <div className={styles.panelHeader}>
            <Heading as="h2">{windowLabels[windowKey]}</Heading>
            <span>Lower is better</span>
          </div>
          {error ? (
            <div className={styles.emptyState}>Benchmark history could not be loaded: {error}</div>
          ) : (
            <TrendChart points={visiblePoints} valueMode={valueMode} />
          )}
        </section>

        <section className={styles.grid}>
          <div className={styles.panel}>
            <Heading as="h2">Latest Values</Heading>
            {latestRows.length === 0 ? (
              <div className={styles.emptyState}>No values match the selected filters.</div>
            ) : (
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Metric</th>
                    <th>Release</th>
                    <th>Value</th>
                    <th>Confidence</th>
                  </tr>
                </thead>
                <tbody>
                  {latestRows.map((point) => (
                    <tr key={point.metric_id}>
                      <td>{point.label}</td>
                      <td>{point.release_tag || point.release_version || 'unknown'}</td>
                      <td>{formatValue(point.value, point.unit, valueMode)}</td>
                      <td>{point.confidence}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className={styles.panel}>
            <Heading as="h2">Environment Notes</Heading>
            {reducedConfidence.length === 0 ? (
              <p>Visible points are normal confidence based on recorded metadata.</p>
            ) : (
              <ul className={styles.notes}>
                {reducedConfidence.slice(0, 8).map((point) => (
                  <li key={`${point.record_key}-${point.metric_id}`}>
                    {point.release_tag || point.release_version}: {point.label} uses {point.source_kind} data with {point.confidence} confidence.
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      </main>
    </Layout>
  );
}
