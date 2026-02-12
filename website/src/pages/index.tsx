import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Translate, {translate} from '@docusaurus/Translate';
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
        <div className={styles.alphaNotice}>
          <span className={styles.alphaBadge}>
            <Translate id="homepage.hero.alpha.badge">ALPHA</Translate>
          </span>
          <span>
            <Translate id="homepage.hero.alpha.notice">
              Invowk™ is in early development. Expect breaking changes between releases.
            </Translate>
          </span>
        </div>
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">
          <Translate id="homepage.hero.tagline" description="The tagline shown on the homepage hero">
            A dynamically extensible command runner. Like `just`, but with superpowers.
          </Translate>
        </p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started/installation">
            <Translate id="homepage.hero.button.getStarted">Get Started</Translate>
          </Link>
          <Link
            className="button button--outline button--lg"
            style={{marginLeft: '1rem', color: 'white', borderColor: 'white'}}
            to="/docs/getting-started/quickstart">
            <Translate id="homepage.hero.button.quickstart">Quickstart</Translate>
          </Link>
        </div>
        <div className={styles.terminalDemo}>
          <pre className={styles.terminalCode}>
            <code>
{`$ invowk init
Created invkfile.cue

$ invowk cmd
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
      title={translate({
        id: 'homepage.layout.title',
        message: 'Dynamically Extensible Command Runner',
      })}
      description={translate({
        id: 'homepage.layout.description',
        message: 'Invowk is a powerful, dynamically extensible command runner similar to just, written in Go. Define commands in CUE format and run them with native shell, virtual shell, or containers.',
      })}>
      <HomepageHeader />
      <main>
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
