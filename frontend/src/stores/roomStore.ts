import { create } from 'zustand';
import { api } from '../api/client';
import type { Room, RoomState } from '../types';

export const useRoomStore = create<RoomState>((set) => ({
  rooms: [],
  currentRoom: null,
  isLoading: false,

  fetchRooms: async () => {
    set({ isLoading: true });
    try {
      const res = await api.rooms.list();
      set({ rooms: res.rooms || [], isLoading: false });
    } catch {
      set({ isLoading: false });
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
