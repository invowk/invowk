import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import Heading from '@theme/Heading';

import styles from './index.module.css';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started/installation">
            Get Started
          </Link>
          <Link
            className="button button--outline button--lg"
            style={{marginLeft: '1rem', color: 'white', borderColor: 'white'}}
            to="/docs/getting-started/quickstart">
            Quickstart
          </Link>
        </div>
        <div className={styles.terminalDemo}>
          <pre className={styles.terminalCode}>
            <code>
{`$ invowk init
Created invkfile.cue

$ invowk cmd --list
Available Commands
  myproject build - Build the project [native*]
  myproject test unit - Run unit tests [native*, virtual]
  myproject deploy - Deploy to production [container*]

$ invowk cmd myproject build
Building project...
Done!`}
            </code>
          </pre>
        </div>
      </div>
    </header>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Dynamically Extensible Command Runner"
      description="Invowk is a powerful, dynamically extensible command runner similar to just, written in Go. Define commands in CUE format and run them with native shell, virtual shell, or containers.">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
