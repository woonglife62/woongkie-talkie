import { useState, useRef, useCallback, ChangeEvent, KeyboardEvent, DragEvent } from 'react';
import { EmojiPicker } from './EmojiPicker';
import { api } from '../api/client';
import type { Message, WSMessage } from '../types';

interface MessageInputProps {
  roomId: string;
  username: string;
  replyTo: Message | null;
  onCancelReply: () => void;
  onSend: (payload: WSMessage) => void;
  onTyping: (isTyping: boolean) => void;
}

export function MessageInput({
  roomId,
  username,
  replyTo,
  onCancelReply,
  onSend,
  onTyping,
}: MessageInputProps) {
  const [text, setText] = useState('');
  const [showEmoji, setShowEmoji] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isTypingRef = useRef(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleTyping = useCallback(() => {
    if (!isTypingRef.current) {
      isTypingRef.current = true;
      onTyping(true);
    }
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    typingTimerRef.current = setTimeout(() => {
      isTypingRef.current = false;
      onTyping(false);
    }, 2000);
  }, [onTyping]);

  const handleChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    setText(e.target.value);
    handleTyping();
    // Auto-resize
    const ta = e.target;
    ta.style.height = 'auto';
    ta.style.height = `${Math.min(ta.scrollHeight, 120)}px`;
  };

  const sendText = useCallback(() => {
    const trimmed = text.trim();
    if (!trimmed) return;

    const payload: WSMessage = {
      Event: 'MSG',
      User: username,
      message: trimmed,
      room_id: roomId,
    };

    if (replyTo) {
      payload.reply_to = replyTo.id;
      payload.reply_to_message = replyTo.message;
      payload.reply_to_user = replyTo.user;
      onCancelReply();
    }

    onSend(payload);
    setText('');
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    isTypingRef.current = false;
    onTyping(false);
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  }, [text, username, roomId, replyTo, onCancelReply, onSend, onTyping]);

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendText();
    }
  };

  const uploadFile = useCallback(
    async (file: File) => {
      setIsUploading(true);
      setUploadError(null);
      try {
        const res = await api.files.upload(roomId, file);
        const payload: WSMessage = {
          Event: 'MSG_FILE',
          User: username,
          message: res.url || `/files/${res.file_id}`,
          room_id: roomId,
        };
        onSend(payload);
      } catch (err) {
        const msg = err instanceof Error ? err.message : '파일 업로드에 실패했습니다.';
        setUploadError(msg);
        setTimeout(() => setUploadError(null), 4000);
      } finally {
        setIsUploading(false);
      }
    },
    [roomId, username, onSend]
  );

  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) uploadFile(file);
    e.target.value = '';
  };

  const handleDragOver = (e: DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = () => setIsDragging(false);

  const handleDrop = (e: DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const file = e.dataTransfer.files?.[0];
    if (file) uploadFile(file);
  };

  const handleEmojiSelect = (emoji: string) => {
    setText((prev) => prev + emoji);
    setShowEmoji(false);
    textareaRef.current?.focus();
  };

  return (
    <div
      className="message-input-area"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      style={{ position: 'relative' }}
    >
      {isDragging && (
        <div className="drag-overlay">파일을 여기에 놓으세요</div>
      )}

      {uploadError && (
        <div style={{ padding: '4px 16px', color: '#dc3545', fontSize: 13, background: 'rgba(220,53,69,0.08)', borderRadius: 4, marginBottom: 4 }}>
          {uploadError}
        </div>
      )}
      {replyTo && (
        <div className="reply-preview">
          <span className="reply-preview-text">
            {replyTo.user}에게 답장: {replyTo.message}
          </span>
          <button className="reply-cancel-btn" onClick={onCancelReply}>
            ✕
          </button>
        </div>
      )}

      <div className="input-row">
        <textarea
          ref={textareaRef}
          value={text}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder="메시지를 입력하세요... (Shift+Enter로 줄바꿈)"
          rows={1}
          disabled={isUploading}
        />
        <button className="send-btn" onClick={sendText} disabled={!text.trim() || isUploading}>
          전송
        </button>
      </div>

      <div className="input-tools" style={{ position: 'relative' }}>
        <button
          className="tool-btn"
          type="button"
          onClick={() => setShowEmoji((v) => !v)}
          title="이모지"
        >
          😊
        </button>
        <button
          className="tool-btn"
          type="button"
          onClick={() => fileInputRef.current?.click()}
          title="파일 업로드"
          disabled={isUploading}
        >
          {isUploading ? '업로드 중...' : '📎 파일'}
        </button>
        <input
          ref={fileInputRef}
          type="file"
          style={{ display: 'none' }}
          onChange={handleFileChange}
        />
        {showEmoji && (
          <EmojiPicker onSelect={handleEmojiSelect} onClose={() => setShowEmoji(false)} />
        )}
      </div>
    </div>
  );
}
