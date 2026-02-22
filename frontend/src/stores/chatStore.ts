import { create } from 'zustand';
import type { ChatState, Message } from '../types';

export const useChatStore = create<ChatState>((set) => ({
  messages: {},
  typingUsers: {},

  addMessage: (roomId: string, message: Message) =>
    set((state) => ({
      messages: {
        ...state.messages,
        [roomId]: [...(state.messages[roomId] || []), message],
      },
    })),

  setMessages: (roomId: string, messages: Message[]) =>
    set((state) => ({
      messages: { ...state.messages, [roomId]: messages },
    })),

  updateMessage: (roomId: string, messageId: string, text: string) =>
    set((state) => ({
      messages: {
        ...state.messages,
        [roomId]: (state.messages[roomId] || []).map((m) =>
          m.id === messageId
            ? { ...m, message: text, is_edited: true, updated_at: new Date().toISOString() }
            : m
        ),
      },
    })),

  deleteMessage: (roomId: string, messageId: string) =>
    set((state) => ({
      messages: {
        ...state.messages,
        [roomId]: (state.messages[roomId] || []).map((m) =>
          m.id === messageId ? { ...m, is_deleted: true, message: '삭제된 메시지입니다.' } : m
        ),
      },
    })),

  setTyping: (roomId: string, user: string, isTyping: boolean) =>
    set((state) => {
      const current = state.typingUsers[roomId] || [];
      const next = isTyping
        ? current.includes(user) ? current : [...current, user]
        : current.filter((u) => u !== user);
      return { typingUsers: { ...state.typingUsers, [roomId]: next } };
    }),
}));
