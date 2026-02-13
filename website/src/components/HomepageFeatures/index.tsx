import type {ReactNode} from 'react';
import clsx from 'clsx';
import Translate from '@docusaurus/Translate';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  titleId: string;
  titleDefault: string;
  icon: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    titleId: 'homepage.features.threeRuntimeModes.title',
    titleDefault: 'Three Runtime Modes',
    icon: '🚀',
    description: (
      <Translate id="homepage.features.threeRuntimeModes.description">
        Run commands with native shell, the built-in virtual shell (cross-platform POSIX), or inside containers (Docker/Podman). Pick what works best for each command.
      </Translate>
    ),
  },
  {
    titleId: 'homepage.features.cuePowered.title',
    titleDefault: 'CUE-Powered Configuration',
    icon: '⚙️',
    description: (
      <Translate id="homepage.features.cuePowered.description">
        Define commands in invowkfile.cue using CUE, a powerful configuration language with built-in validation. Say goodbye to YAML indentation nightmares.
      </Translate>
    ),
  },
  {
    titleId: 'homepage.features.smartDependencies.title',
    titleDefault: 'Smart Dependencies',
    icon: '🔗',
    description: (
      <Translate id="homepage.features.smartDependencies.description">
        Declare tool, file, capability, and environment dependencies. Invowk validates everything before running, giving you clear error messages when something is missing.
      </Translate>
    ),
  },
  {
    titleId: 'homepage.features.crossPlatform.title',
    titleDefault: 'Cross-Platform',
    icon: '🌍',
    description: (
      <Translate id="homepage.features.crossPlatform.description">
        Works on Linux, macOS, and Windows. Write platform-specific implementations for the same command, and Invowk picks the right one automatically.
      </Translate>
    ),
  },
  {
    titleId: 'homepage.features.interactiveTui.title',
    titleDefault: 'Interactive TUI',
    icon: '🎨',
    description: (
      <Translate id="homepage.features.interactiveTui.description">
        Built-in terminal UI components (like gum) for creating interactive scripts: input prompts, selections, confirmations, spinners, and more.
      </Translate>
    ),
  },
  {
    titleId: 'homepage.features.distributableModules.title',
    titleDefault: 'Distributable Modules',
    icon: '📦',
    description: (
      <Translate id="homepage.features.distributableModules.description">
        Bundle commands and scripts into modules for easy sharing and distribution. Import modules from files or URLs with a single command.
      </Translate>
    ),
  },
];

function Feature({titleId, titleDefault, icon, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className={clsx('text--center padding--lg', styles.featureCard)}>
        <div className={styles.featureIcon}>{icon}</div>
        <Heading as="h3">
          <Translate id={titleId}>{titleDefault}</Translate>
        </Heading>
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
