import { useEffect, useRef, useCallback } from 'react';
import { useChatStore } from '../stores/chatStore';
import type { WSMessage, Message } from '../types';

const OFFLINE_QUEUE_KEY = 'ws_offline_queue';

function getOfflineQueue(): WSMessage[] {
  try {
    return JSON.parse(sessionStorage.getItem(OFFLINE_QUEUE_KEY) || '[]');
  } catch {
    return [];
  }
}

function saveOfflineQueue(queue: WSMessage[]) {
  sessionStorage.setItem(OFFLINE_QUEUE_KEY, JSON.stringify(queue));
}

export function useWebSocket(roomId: string | null, username: string | null) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttempts = useRef(0);
  const { addMessage, updateMessage, deleteMessage, setTyping } = useChatStore();

  const flushOfflineQueue = useCallback((ws: WebSocket) => {
    const queue = getOfflineQueue();
    if (queue.length === 0) return;
    // Only flush messages for the current room (#175: filter by room_id)
    const remaining: WSMessage[] = [];
    queue.forEach((msg) => {
      if (msg.room_id === roomId && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(msg));
      } else {
        remaining.push(msg);
      }
    });
    saveOfflineQueue(remaining);
  }, [roomId]);

  const connect = useCallback(() => {
    if (!roomId || !username) return;
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/rooms/${roomId}/ws`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      reconnectAttempts.current = 0;
      flushOfflineQueue(ws);
    };

    ws.onmessage = (event) => {
      let data: WSMessage;
      try {
        data = JSON.parse(event.data);
      } catch {
        return;
      }

      switch (data.Event) {
        case 'MSG': {
          if (!data.User || data.message == null) break;
          const msg: Message = {
            id: data._id || data.message_id || `${Date.now()}-${Math.random()}`,
            room_id: roomId,
            user: data.User,
            message: data.message,
            created_at: data.created_at || new Date().toISOString(),
            reply_to: data.reply_to,
            reply_to_message: data.reply_to_message,
            reply_to_user: data.reply_to_user,
          };
          addMessage(roomId, msg);
          setTyping(roomId, data.User, false);
          break;
        }
        case 'MSG_FILE': {
          if (!data.User || data.message == null) break;
          const msg: Message = {
            id: data._id || data.message_id || `${Date.now()}-${Math.random()}`,
            room_id: roomId,
            user: data.User,
            message: data.message,
            created_at: data.created_at || new Date().toISOString(),
            file_url: data.message,
          };
          addMessage(roomId, msg);
          break;
        }
        case 'MSG_EDIT': {
          if (!data.message_id || data.message == null) break;
          updateMessage(roomId, data.message_id, data.message);
          break;
        }
        case 'MSG_DELETE': {
          if (!data.message_id) break;
          deleteMessage(roomId, data.message_id);
          break;
        }
        case 'TYPING_START': {
          if (data.User && data.User !== username) {
            setTyping(roomId, data.User, true);
          }
          break;
        }
        case 'TYPING_STOP': {
          if (data.User) {
            setTyping(roomId, data.User, false);
          }
          break;
        }
      }
    };

    ws.onclose = () => {
      wsRef.current = null;
      const delay = Math.min(1000 * 2 ** reconnectAttempts.current, 30000);
      reconnectAttempts.current += 1;
      reconnectTimerRef.current = setTimeout(() => {
        connect();
      }, delay);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [roomId, username, addMessage, updateMessage, deleteMessage, setTyping, flushOfflineQueue]);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect]);

  const sendMessage = useCallback(
    (payload: WSMessage) => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify(payload));
      } else {
        const queue = getOfflineQueue();
        queue.push(payload);
        saveOfflineQueue(queue);
      }
    },
    []
  );

  const sendTyping = useCallback(
    (isTyping: boolean) => {
      if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
      wsRef.current.send(
        JSON.stringify({
          Event: isTyping ? 'TYPING_START' : 'TYPING_STOP',
          User: username,
          room_id: roomId,
        })
      );
    },
    [roomId, username]
  );

  return { sendMessage, sendTyping };
}
