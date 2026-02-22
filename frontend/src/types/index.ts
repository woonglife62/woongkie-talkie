export interface User {
  username: string;
  display_name?: string;
  bio?: string;
  avatar?: string;
}

export interface Room {
  id: string;
  name: string;
  description?: string;
  created_by: string;
  member_count: number;
  created_at: string;
}

export interface Message {
  id: string;
  room_id: string;
  user: string;
  display_name?: string;
  message: string;
  created_at: string;
  updated_at?: string;
  is_edited?: boolean;
  is_deleted?: boolean;
  reply_to?: string;
  reply_to_message?: string;
  reply_to_user?: string;
  file_url?: string;
}

export interface WSMessage {
  Event: string;
  User?: string;
  message?: string;
  room_id?: string;
  message_id?: string;
  _id?: string;
  created_at?: string;
  reply_to?: string;
  reply_to_message?: string;
  reply_to_user?: string;
}

export interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  fetchMe: () => Promise<void>;
}

export interface ChatState {
  messages: Record<string, Message[]>;
  typingUsers: Record<string, string[]>;
  addMessage: (roomId: string, message: Message) => void;
  setMessages: (roomId: string, messages: Message[]) => void;
  updateMessage: (roomId: string, messageId: string, text: string) => void;
  deleteMessage: (roomId: string, messageId: string) => void;
  setTyping: (roomId: string, user: string, isTyping: boolean) => void;
}

export interface RoomState {
  rooms: Room[];
  currentRoom: Room | null;
  isLoading: boolean;
  error: string | null;
  fetchRooms: () => Promise<void>;
  joinRoom: (roomId: string) => Promise<void>;
  leaveRoom: (roomId: string) => Promise<void>;
  createRoom: (name: string, description?: string) => Promise<Room>;
  deleteRoom: (roomId: string) => Promise<void>;
  setCurrentRoom: (room: Room | null) => void;
}
