import { useState, useEffect } from 'react';
import { Login } from './components/Login';
import { ChatRoom } from './components/ChatRoom';
import { RoomSidebar } from './components/RoomSidebar';
import { useAuthStore } from './stores/authStore';
import { useRoomStore } from './stores/roomStore';
import type { Room } from './types';

export default function App() {
  const { isAuthenticated, isLoading, user, fetchMe } = useAuthStore();
  const { setCurrentRoom, currentRoom } = useRoomStore();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem('auth_token');
    if (token && !user) {
      fetchMe();
    }
  }, []);

  if (isLoading) {
    return <div className="spinner">로딩 중...</div>;
  }

  if (!isAuthenticated) {
    return <Login />;
  }

  const handleSelectRoom = (room: Room) => {
    setCurrentRoom(room);
    setSidebarOpen(false);
  };

  return (
    <div className="app">
      <RoomSidebar
        currentRoomId={currentRoom?.id ?? null}
        onSelectRoom={handleSelectRoom}
        isOpen={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
      />
      <ChatRoom
        username={user?.username ?? ''}
        onToggleSidebar={() => setSidebarOpen((v) => !v)}
      />
    </div>
  );
}
