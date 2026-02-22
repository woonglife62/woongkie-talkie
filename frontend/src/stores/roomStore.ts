import { create } from 'zustand';
import { api } from '../api/client';
import type { Room, RoomState } from '../types';

export const useRoomStore = create<RoomState>((set) => ({
  rooms: [],
  currentRoom: null,
  isLoading: false,
  error: null,

  fetchRooms: async () => {
    set({ isLoading: true, error: null });
    try {
      const res = await api.rooms.list();
      set({ rooms: res.rooms || [], isLoading: false });
    } catch (err) {
      set({ isLoading: false, error: err instanceof Error ? err.message : '채팅방 목록을 불러오지 못했습니다.' });
    }
  },

  joinRoom: async (roomId: string) => {
    await api.rooms.join(roomId);
  },

  leaveRoom: async (roomId: string) => {
    await api.rooms.leave(roomId);
    set((state) => ({
      currentRoom: state.currentRoom?.id === roomId ? null : state.currentRoom,
    }));
  },

  createRoom: async (name: string, description?: string): Promise<Room> => {
    const room = await api.rooms.create(name, description);
    set((state) => ({ rooms: [...state.rooms, room] }));
    return room;
  },

  deleteRoom: async (roomId: string) => {
    await api.rooms.delete(roomId);
    set((state) => ({
      rooms: state.rooms.filter((r) => r.id !== roomId),
      currentRoom: state.currentRoom?.id === roomId ? null : state.currentRoom,
    }));
  },

  setCurrentRoom: (room: Room | null) => set({ currentRoom: room }),
}));
