import { useState, useEffect, useCallback } from 'react';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';
import { useWebSocket } from '../hooks/useWebSocket';
import { useChatStore } from '../stores/chatStore';
import { useRoomStore } from '../stores/roomStore';
import { api } from '../api/client';
import type { Message, WSMessage } from '../types';

interface SearchBarProps {
  roomId: string;
  onClose: () => void;
}

function SearchBar({ roomId, onClose }: SearchBarProps) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Message[]>([]);
  const [searching, setSearching] = useState(false);

  const handleSearch = async () => {
    if (!query.trim()) return;
    setSearching(true);
    try {
      const res = await api.messages.search(roomId, query.trim());
      setResults(res.messages || []);
    } catch {
      setResults([]);
    }
    setSearching(false);
  };

  const highlight = (text: string, q: string) => {
    if (!q) return text;
    const idx = text.toLowerCase().indexOf(q.toLowerCase());
    if (idx === -1) return text;
    return (
      <>
        {text.slice(0, idx)}
        <mark className="search-highlight">{text.slice(idx, idx + q.length)}</mark>
        {text.slice(idx + q.length)}
      </>
    );
  };

  return (
    <div className="search-bar">
      <input
        className="search-input"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
        placeholder="메시지 검색..."
        autoFocus
      />
      <button className="msg-action-btn" onClick={handleSearch} disabled={searching}>
        {searching ? '...' : '검색'}
      </button>
      <button className="msg-action-btn" onClick={onClose}>✕</button>
      {results.length > 0 && (
        <div className="search-results">
          {results.map((msg) => (
            <div key={msg.id} className="search-result-item">
              <span className="search-result-user">{msg.user}</span>
              {highlight(msg.message, query)}
            </div>
          ))}
        </div>
      )}
      {results.length === 0 && query && !searching && (
        <div className="search-results">
          <div className="search-result-item">검색 결과가 없습니다.</div>
        </div>
      )}
    </div>
  );
}

interface ChatRoomProps {
  username: string;
  onToggleSidebar: () => void;
}

export function ChatRoom({ username, onToggleSidebar }: ChatRoomProps) {
  const currentRoom = useRoomStore((s) => s.currentRoom);
  const { setMessages } = useChatStore();
  const typingUsers = useChatStore((s) =>
    currentRoom ? (s.typingUsers[currentRoom.id] || []) : []
  );
  const [replyTo, setReplyTo] = useState<Message | null>(null);
  const [showSearch, setShowSearch] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const { sendMessage, sendTyping } = useWebSocket(
    currentRoom?.id ?? null,
    username
  );

  // Load message history when room changes
  useEffect(() => {
    if (!currentRoom) return;
    setReplyTo(null);
    setLoadError(null);
    api.messages
      .list(currentRoom.id)
      .then((res) => setMessages(currentRoom.id, res.messages || []))
      .catch(() => setLoadError('메시지를 불러오지 못했습니다. 새로고침해 주세요.'));
  }, [currentRoom?.id]);

  const handleSend = useCallback(
    (payload: WSMessage) => {
      sendMessage(payload);
    },
    [sendMessage]
  );

  const handleTyping = useCallback(
    (isTyping: boolean) => {
      sendTyping(isTyping);
    },
    [sendTyping]
  );

  if (!currentRoom) {
    return (
      <div className="main-chat">
        <div className="chat-header">
          <button
            className="btn-icon sidebar-toggle"
            style={{ marginRight: 8 }}
            onClick={onToggleSidebar}
          >
            ☰
          </button>
        </div>
        <div className="no-room">채팅방을 선택해주세요</div>
      </div>
    );
  }

  const typingText =
    typingUsers.length > 0
      ? `${typingUsers.join(', ')}님이 입력 중...`
      : '';

  return (
    <div className="main-chat">
      <div className="chat-header">
        <div style={{ display: 'flex', alignItems: 'center', minWidth: 0 }}>
          <button
            className="btn-icon sidebar-toggle"
            style={{ marginRight: 8 }}
            onClick={onToggleSidebar}
          >
            ☰
          </button>
          <div className="room-info">
            <span className="room-name"># {currentRoom.name}</span>
            {currentRoom.description && (
              <span className="room-desc">{currentRoom.description}</span>
            )}
          </div>
        </div>
        <div className="chat-header-right">
          <span className="member-count">{currentRoom.member_count}명</span>
          <button
            className="btn-icon"
            onClick={() => setShowSearch((v) => !v)}
            title="메시지 검색"
            style={{ fontSize: 14 }}
          >
            🔍
          </button>
        </div>
      </div>

      {showSearch && (
        <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--color-border)', background: 'var(--bg-header)' }}>
          <SearchBar roomId={currentRoom.id} onClose={() => setShowSearch(false)} />
        </div>
      )}

      <div className="chat-body" style={{ flexDirection: 'column' }}>
        {loadError && (
          <div style={{ padding: '8px 16px', background: 'rgba(220,53,69,0.1)', color: '#dc3545', fontSize: 13, flexShrink: 0 }}>
            {loadError}
          </div>
        )}
        <MessageList
          roomId={currentRoom.id}
          currentUser={username}
          onReply={setReplyTo}
        />
      </div>

      <div className="typing-indicator">{typingText}</div>

      <MessageInput
        roomId={currentRoom.id}
        username={username}
        replyTo={replyTo}
        onCancelReply={() => setReplyTo(null)}
        onSend={handleSend}
        onTyping={handleTyping}
      />
    </div>
  );
}
