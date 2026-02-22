import { useState, useEffect } from 'react';
import { useRoomStore } from '../stores/roomStore';
import type { Room } from '../types';

interface RoomListProps {
  currentRoomId: string | null;
  onSelectRoom: (room: Room) => void;
}

export function RoomList({ currentRoomId, onSelectRoom }: RoomListProps) {
  const { rooms, fetchRooms, isLoading, error } = useRoomStore();

  useEffect(() => {
    fetchRooms();
  }, [fetchRooms]);

  if (isLoading && rooms.length === 0) {
    return <div className="room-list" style={{ padding: '16px', color: 'var(--color-sidebar-muted)' }}>로딩 중...</div>;
  }

  if (error && rooms.length === 0) {
    return (
      <div className="room-list" style={{ padding: '16px', color: '#dc3545', fontSize: 13 }}>
        {error}
      </div>
    );
  }

  if (rooms.length === 0) {
    return (
      <div className="room-list" style={{ padding: '16px', color: 'var(--color-sidebar-muted)', fontSize: 13 }}>
        채팅방이 없습니다.
      </div>
    );
  }

  return (
    <div className="room-list">
      {rooms.map((room) => (
        <div
          key={room.id}
          className={`room-item ${room.id === currentRoomId ? 'active' : ''}`}
          onClick={() => onSelectRoom(room)}
        >
          <div className="room-item-name"># {room.name}</div>
          <div className="room-item-info">
            <span className="room-item-members">{room.member_count}명</span>
            {room.description && (
              <span style={{ marginLeft: 6 }}>{room.description}</span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
