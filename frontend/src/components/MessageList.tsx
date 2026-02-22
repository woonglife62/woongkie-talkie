import { useRef, useEffect, useState, useCallback } from 'react';
import { useChatStore } from '../stores/chatStore';
import { api } from '../api/client';
import type { Message } from '../types';

interface MessageListProps {
  roomId: string;
  currentUser: string;
  onReply: (msg: Message) => void;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' });
}

function isImageUrl(url: string): boolean {
  return /\.(png|jpe?g|gif|webp|svg)$/i.test(url) || url.includes('/files/');
}

function canEdit(createdAt: string): boolean {
  return Date.now() - new Date(createdAt).getTime() < 5 * 60 * 1000;
}

interface EditState {
  id: string;
  text: string;
}

export function MessageList({ roomId, currentUser, onReply }: MessageListProps) {
  const messages = useChatStore((s) => s.messages[roomId] || []);
  const { updateMessage, deleteMessage } = useChatStore();
  const bottomRef = useRef<HTMLDivElement>(null);
  const [editState, setEditState] = useState<EditState | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages.length]);

  // Auto-clear action error after 3 seconds
  useEffect(() => {
    if (!actionError) return;
    const t = setTimeout(() => setActionError(null), 3000);
    return () => clearTimeout(t);
  }, [actionError]);

  const handleEdit = useCallback(async (msg: Message) => {
    if (!editState) return;
    try {
      await api.messages.edit(roomId, msg.id, editState.text);
      updateMessage(roomId, msg.id, editState.text);
      setEditState(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '메시지 수정에 실패했습니다.');
      setEditState(null);
    }
  }, [editState, roomId, updateMessage]);

  const handleDelete = useCallback(async (msgId: string) => {
    if (!confirm('메시지를 삭제하시겠습니까?')) return;
    try {
      await api.messages.delete(roomId, msgId);
      deleteMessage(roomId, msgId);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '메시지 삭제에 실패했습니다.');
    }
  }, [roomId, deleteMessage]);

  return (
    <div className="messages-area">
      {actionError && (
        <div className="msg-alert" style={{ color: '#dc3545', padding: '6px 16px', background: 'rgba(220,53,69,0.08)', borderRadius: 6, margin: '4px 16px' }}>
          {actionError}
        </div>
      )}
      {messages.map((msg) => {
        const isMe = msg.user === currentUser;
        const cls = isMe ? 'msg-me' : 'msg-you';

        if (msg.is_deleted) {
          return (
            <div key={msg.id} className={`msg-wrapper ${cls}`}>
              <span className="msg-bubble msg-deleted">삭제된 메시지입니다.</span>
            </div>
          );
        }

        return (
          <div key={msg.id} className={`msg-wrapper ${cls}`}>
            {!isMe && (
              <div className="msg-username">{msg.display_name || msg.user}</div>
            )}
            {msg.reply_to && (
              <span className="reply-quote" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block', maxWidth: 'min(300px, 70vw)' }}>
                {msg.reply_to_user}: {msg.reply_to_message}
              </span>
            )}

            {editState?.id === msg.id ? (
              <div style={{ display: 'inline-flex', gap: 4, maxWidth: 'min(420px, 80vw)' }}>
                <input
                  className="edit-input"
                  value={editState.text}
                  onChange={(e) => setEditState({ ...editState, text: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleEdit(msg);
                    if (e.key === 'Escape') setEditState(null);
                  }}
                  autoFocus
                />
                <button className="msg-action-btn" onClick={() => handleEdit(msg)}>저장</button>
                <button className="msg-action-btn" onClick={() => setEditState(null)}>취소</button>
              </div>
            ) : (
              <span className={`msg-bubble ${isMe ? 'msg-bubble-me' : 'msg-bubble-you'}`}>
                {isImageUrl(msg.message) ? (
                  <img
                    src={msg.message}
                    alt="file"
                    className="msg-file-img"
                    onError={(e) => {
                      (e.target as HTMLImageElement).style.display = 'none';
                    }}
                  />
                ) : (
                  msg.message
                )}
                {msg.is_edited && <span className="msg-edited">(수정됨)</span>}
              </span>
            )}

            <div className="msg-timestamp">{formatTime(msg.created_at)}</div>

            {/* Actions */}
            <div className="msg-actions">
              <button className="msg-action-btn" onClick={() => onReply(msg)}>
                답장
              </button>
              {isMe && canEdit(msg.created_at) && (
                <button
                  className="msg-action-btn"
                  onClick={() => setEditState({ id: msg.id, text: msg.message })}
                >
                  수정
                </button>
              )}
              {isMe && (
                <button
                  className="msg-action-btn danger"
                  onClick={() => handleDelete(msg.id)}
                >
                  삭제
                </button>
              )}
            </div>
          </div>
        );
      })}
      <div ref={bottomRef} />
    </div>
  );
}
