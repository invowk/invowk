import type {ReactNode} from 'react';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import Link from '@docusaurus/Link';
import styles from './performance.module.css';

const bencherPerfUrl = 'https://bencher.dev/perf/invowk';

const trackedMetrics = [
  'CLI startup latency for version, help, command help, and command listing paths',
  'Parser, discovery, module validation, and end-to-end command pipeline latency',
  'Go benchmark memory usage and allocation counts',
  'Release build time and stripped binary size',
];

export default function Performance(): ReactNode {
  return (
    <Layout title="Performance" description="Release performance history for Invowk.">
      <main className={styles.page}>
        <section className={styles.header}>
          <Heading as="h1">Performance</Heading>
          <p>
            Release performance is tracked in Bencher by version tag, so users can compare published versions directly instead of reading pull request benchmark noise.
          </p>
          <div className={styles.actions}>
            <Link className="button button--primary" to={bencherPerfUrl}>
              Open Bencher Performance History
            </Link>
          </div>
        </section>

        <section className={styles.section}>
          <Heading as="h2">Release Tracking</Heading>
          <p>
            Every release tag, such as <code>v0.13.0</code>, is recorded as a Bencher branch with the release commit hash. The new tag starts from the previous release tag, preserving version-to-version history for the public performance page.
          </p>
        </section>

        <section className={styles.grid}>
          <div className={styles.section}>
            <Heading as="h2">Tracked Metrics</Heading>
            <ul className={styles.notes}>
              {trackedMetrics.map((metric) => (
                <li key={metric}>{metric}</li>
              ))}
            </ul>
          </div>

          <div className={styles.section}>
            <Heading as="h2">Maintainer Workflow</Heading>
            <p>
              Pull requests use Bencher as a regression gate. Releases use the Git tag as the durable public performance identity.
            </p>
            <p>
              Local benchmark data can be generated with <code>make bench-bmf</code>; CI uploads that Bencher Metric Format JSON with Bencher's <code>json</code> adapter.
            </p>
          </div>
        </section>
      </main>
    </Layout>
  );
}
