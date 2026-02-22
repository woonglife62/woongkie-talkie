import { useState } from 'react';
import { RoomList } from './RoomList';
import { useRoomStore } from '../stores/roomStore';
import { useAuthStore } from '../stores/authStore';
import type { Room } from '../types';

interface RoomSidebarProps {
  currentRoomId: string | null;
  onSelectRoom: (room: Room) => void;
  isOpen: boolean;
  onClose: () => void;
}

function CreateRoomModal({ onClose }: { onClose: () => void }) {
  const [name, setName] = useState('');
  const [desc, setDesc] = useState('');
  const [error, setError] = useState('');
  const { createRoom, setCurrentRoom } = useRoomStore();

  const handleCreate = async () => {
    if (!name.trim()) { setError('방 이름을 입력해주세요.'); return; }
    try {
      const room = await createRoom(name.trim(), desc.trim() || undefined);
      setCurrentRoom(room);
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '생성 실패');
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>새 채팅방 만들기</h3>
        <div className="form-group">
          <label>방 이름</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="채팅방 이름"
            autoFocus
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          />
        </div>
        <div className="form-group">
          <label>설명 (선택)</label>
          <input
            type="text"
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            placeholder="채팅방 설명"
          />
        </div>
        {error && <p className="error-msg">{error}</p>}
        <div className="modal-actions">
          <button className="btn btn-secondary" onClick={onClose}>취소</button>
          <button className="btn btn-primary" onClick={handleCreate}>만들기</button>
        </div>
      </div>
    </div>
  );
}

export function RoomSidebar({ currentRoomId, onSelectRoom, isOpen, onClose }: RoomSidebarProps) {
  const [showCreate, setShowCreate] = useState(false);
  const { user, logout } = useAuthStore();

  const toggleTheme = () => {
    const html = document.documentElement;
    const isDark = html.getAttribute('data-theme') === 'dark';
    if (isDark) {
      html.removeAttribute('data-theme');
      localStorage.setItem('theme', 'light');
    } else {
      html.setAttribute('data-theme', 'dark');
      localStorage.setItem('theme', 'dark');
    }
  };

  const isDark = document.documentElement.getAttribute('data-theme') === 'dark';

  return (
    <>
      {isOpen && <div className="sidebar-backdrop" style={{ display: 'block' }} onClick={onClose} />}
      <div className={`sidebar${isOpen ? ' sidebar-open' : ''}`}>
        <div className="sidebar-header">
          <h5>Woongkie-Talkie</h5>
          <div className="sidebar-actions">
            <button
              className="theme-toggle"
              onClick={toggleTheme}
              title={isDark ? '라이트 모드' : '다크 모드'}
            >
              {isDark ? '☀️' : '🌙'}
            </button>
            <button
              className="btn-icon"
              onClick={() => setShowCreate(true)}
              title="새 채팅방"
              style={{ fontSize: 20, fontWeight: 'bold' }}
            >
              +
            </button>
          </div>
        </div>

        <RoomList currentRoomId={currentRoomId} onSelectRoom={(r) => { onSelectRoom(r); onClose(); }} />

        <div className="sidebar-footer">
          <div className="profile-info">
            <span className="profile-display-name">{user?.display_name || user?.username}</span>
            {user?.bio && <span className="profile-bio">{user.bio}</span>}
          </div>
          <button
            className="btn-icon"
            onClick={logout}
            title="로그아웃"
            style={{ fontSize: 14, color: 'var(--color-sidebar-muted)' }}
          >
            ↪
          </button>
        </div>
      </div>

      {showCreate && <CreateRoomModal onClose={() => setShowCreate(false)} />}
    </>
  );
}
