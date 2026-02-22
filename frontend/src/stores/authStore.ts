import { create } from 'zustand';
import { api } from '../api/client';
import type { AuthState, User } from '../types';

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  isLoading: true,

  login: async (username: string, password: string) => {
    set({ isLoading: true });
    try {
      const res = await api.auth.login(username, password);
      set({
        user: { username: res.user.username },
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  register: async (username: string, password: string) => {
    set({ isLoading: true });
    try {
      const res = await api.auth.register(username, password);
      set({
        user: { username: res.user.username },
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  logout: async () => {
    try {
      await api.auth.logout();
    } catch {
      // ignore
    }
    set({ user: null, isAuthenticated: false });
  },

  fetchMe: async () => {
    set({ isLoading: true });
    try {
      const user: User = await api.auth.me();
      set({ user, isAuthenticated: true, isLoading: false });
    } catch {
      set({ user: null, isAuthenticated: false, isLoading: false });
    }
  },
}));
