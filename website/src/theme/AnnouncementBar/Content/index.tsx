/**
 * Custom AnnouncementBar Content component with i18n support.
 * Swizzled from @docusaurus/theme-classic to enable translation of the announcement bar content.
 */

import React, {type ReactNode} from 'react';
import clsx from 'clsx';
import {translate} from '@docusaurus/Translate';
import type {Props} from '@theme/AnnouncementBar/Content';
import styles from './styles.module.css';

export default function AnnouncementBarContent(props: Props): ReactNode {
  const content = translate({
    id: 'theme.announcementBar.content',
    message:
      '⚠️ <strong>Alpha Software</strong> — Invowk is under active development. The invowkfile format and features may change between releases. <a target="_blank" rel="noopener noreferrer" href="https://github.com/invowk/invowk">Star us on GitHub</a> to follow progress!',
    description: 'The content of the announcement bar',
  });

  return (
    <div
      {...props}
      className={clsx(styles.content, props.className)}
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={{__html: content}}
    />
  );
}
