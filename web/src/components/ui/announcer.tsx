'use client';

import { useEffect, useState } from 'react';

interface AnnouncerProps {
  readonly message: string;
  readonly priority?: 'polite' | 'assertive';
}

export function Announcer({ message, priority = 'polite' }: AnnouncerProps) {
  const [announced, setAnnounced] = useState('');

  useEffect(() => {
    if (message) {
      setAnnounced('');
      // Brief delay ensures screen reader picks up the change
      const timer = setTimeout(() => setAnnounced(message), 50);
      return () => clearTimeout(timer);
    }
  }, [message]);

  return (
    <div
      aria-live={priority}
      aria-atomic="true"
      role="status"
      className="sr-only"
      style={{
        position: 'absolute',
        width: '1px',
        height: '1px',
        padding: 0,
        margin: '-1px',
        overflow: 'hidden',
        clip: 'rect(0, 0, 0, 0)',
        whiteSpace: 'nowrap',
        borderWidth: 0,
      }}
    >
      {announced}
    </div>
  );
}
