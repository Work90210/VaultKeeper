'use client';

import { useEffect, useRef, useState } from 'react';
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';

interface UseYjsOptions {
  evidenceId: string;
  accessToken: string;
  username?: string;
  enabled?: boolean;
}

interface UseYjsReturn {
  yDoc: Y.Doc;
  yRedactions: Y.Array<Y.Map<unknown>>;
  awareness: WebsocketProvider['awareness'] | null;
  connected: boolean;
  undoManager: Y.UndoManager;
}

export function useYjs({ evidenceId, accessToken, username = 'User', enabled = true }: UseYjsOptions): UseYjsReturn {
  const [connected, setConnected] = useState(false);
  const [awareness, setAwareness] = useState<WebsocketProvider['awareness'] | null>(null);

  // Stable Y.Doc and derived objects — created once, destroyed on unmount
  const yDocRef = useRef<Y.Doc | null>(null);
  if (!yDocRef.current) yDocRef.current = new Y.Doc();
  const yDoc = yDocRef.current;

  const yRedactionsRef = useRef<Y.Array<Y.Map<unknown>> | null>(null);
  if (!yRedactionsRef.current) yRedactionsRef.current = yDoc.getArray<Y.Map<unknown>>('redactions');
  const yRedactions = yRedactionsRef.current;

  const undoManagerRef = useRef<Y.UndoManager | null>(null);
  if (!undoManagerRef.current) undoManagerRef.current = new Y.UndoManager(yRedactions);
  const undoManager = undoManagerRef.current;

  // Cleanup Y.Doc on unmount
  useEffect(() => {
    return () => {
      undoManagerRef.current?.destroy();
      undoManagerRef.current = null;
      yRedactionsRef.current = null;
      yDocRef.current?.destroy();
      yDocRef.current = null;
    };
  }, []);

  // WebSocket provider lifecycle
  useEffect(() => {
    if (!enabled || !accessToken) return;

    const apiBase = process.env.NEXT_PUBLIC_API_URL || '';
    const wsBase = apiBase.replace(/^http/, 'ws');
    // y-websocket constructs URL as: serverUrl + '/' + roomname + '?' + params
    // So we use the base as server, the full path as room, and token as params
    const provider = new WebsocketProvider(
      wsBase,
      `api/evidence/${evidenceId}/redact/collaborate`,
      yDoc,
      {
        connect: true,
        params: { token: accessToken },
      }
    );

    provider.on('status', ({ status }: { status: string }) => {
      setConnected(status === 'connected');
    });

    // Set awareness state with actual username
    provider.awareness.setLocalStateField('user', {
      name: username,
    });

    setAwareness(provider.awareness);

    return () => {
      provider.disconnect();
      provider.destroy();
      setAwareness(null);
      setConnected(false);
    };
  }, [evidenceId, accessToken, enabled, yDoc, username]);

  return {
    yDoc,
    yRedactions,
    awareness,
    connected,
    undoManager,
  };
}
