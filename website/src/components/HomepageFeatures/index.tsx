import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  icon: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Three Runtime Modes',
    icon: '🚀',
    description: (
      <>
        Run commands with <strong>native</strong> shell, the built-in <strong>virtual</strong> shell 
        (cross-platform POSIX), or inside <strong>containers</strong> (Docker/Podman). 
        Pick what works best for each command.
      </>
    ),
  },
  {
    title: 'CUE-Powered Configuration',
    icon: '⚙️',
    description: (
      <>
        Define commands in <code>invkfile.cue</code> using CUE, a powerful configuration 
        language with built-in validation. Say goodbye to YAML indentation nightmares.
      </>
    ),
  },
  {
    title: 'Smart Dependencies',
    icon: '🔗',
    description: (
      <>
        Declare tool, file, capability, and environment dependencies. Invowk validates 
        everything before running, giving you clear error messages when something's missing.
      </>
    ),
  },
  {
    title: 'Cross-Platform',
    icon: '🌍',
    description: (
      <>
        Works on Linux, macOS, and Windows. Write platform-specific implementations 
        for the same command, and Invowk picks the right one automatically.
      </>
    ),
  },
  {
    title: 'Interactive TUI',
    icon: '🎨',
    description: (
      <>
        Built-in terminal UI components (like <code>gum</code>) for creating interactive 
        scripts: input prompts, selections, confirmations, spinners, and more.
      </>
    ),
  },
  {
    title: 'Distributable Packs',
    icon: '📦',
    description: (
      <>
        Bundle commands and scripts into <strong>packs</strong> for easy sharing 
        and distribution. Import packs from files or URLs with a single command.
      </>
    ),
  },
];

function Feature({title, icon, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className={clsx('text--center padding--lg', styles.featureCard)}>
        <div className={styles.featureIcon}>{icon}</div>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
