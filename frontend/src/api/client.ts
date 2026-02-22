const BASE_URL = '';

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Requested-With': 'XMLHttpRequest',
    ...(options.headers as Record<string, string>),
  };

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
    credentials: 'include',
  });

  if (!res.ok) {
    let errMsg = `HTTP ${res.status}`;
    try {
      const errBody = await res.json();
      errMsg = errBody.message || errBody.error || errMsg;
    } catch {
      // ignore parse error
    }
    throw new Error(errMsg);
  }

  const text = await res.text();
  if (!text) return {} as T;
  return JSON.parse(text) as T;
}

export const api = {
  auth: {
    login: (username: string, password: string) =>
      request<{ token: string; user: { username: string } }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      }),
    register: (username: string, password: string) =>
      request<{ token: string; user: { username: string } }>('/auth/register', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      }),
    logout: () =>
      request<void>('/auth/logout', { method: 'POST' }),
    me: () =>
      request<{ username: string; display_name?: string; bio?: string }>('/auth/me'),
  },

  rooms: {
    list: () =>
      request<{ rooms: import('../types').Room[] }>('/rooms'),
    get: (id: string) =>
      request<import('../types').Room>(`/rooms/${id}`),
    create: (name: string, description?: string) =>
      request<import('../types').Room>('/rooms', {
        method: 'POST',
        body: JSON.stringify({ name, description }),
      }),
    delete: (id: string) =>
      request<void>(`/rooms/${id}`, { method: 'DELETE' }),
    join: (id: string) =>
      request<void>(`/rooms/${id}/join`, { method: 'POST' }),
    leave: (id: string) =>
      request<void>(`/rooms/${id}/leave`, { method: 'POST' }),
  },

  messages: {
    list: (roomId: string, before?: string, limit = 50) => {
      const params = new URLSearchParams({ limit: String(limit) });
      if (before) params.set('before', before);
      return request<{ messages: import('../types').Message[] }>(
        `/rooms/${roomId}/messages?${params}`
      );
    },
    search: (roomId: string, q: string) =>
      request<{ messages: import('../types').Message[] }>(
        `/rooms/${roomId}/messages/search?q=${encodeURIComponent(q)}`
      ),
    edit: (roomId: string, msgId: string, message: string) =>
      request<void>(`/rooms/${roomId}/messages/${msgId}`, {
        method: 'PUT',
        body: JSON.stringify({ message }),
      }),
    delete: (roomId: string, msgId: string) =>
      request<void>(`/rooms/${roomId}/messages/${msgId}`, { method: 'DELETE' }),
    reply: (roomId: string, msgId: string, message: string) =>
      request<void>(`/rooms/${roomId}/messages/${msgId}/reply`, {
        method: 'POST',
        body: JSON.stringify({ message }),
      }),
  },

  files: {
    upload: (roomId: string, file: File) => {
      const formData = new FormData();
      formData.append('file', file);
      return fetch(`/rooms/${roomId}/upload`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'X-Requested-With': 'XMLHttpRequest' },
        body: formData,
      }).then(async (res) => {
        if (!res.ok) throw new Error(`Upload failed: ${res.status}`);
        return res.json() as Promise<{ file_id: string; url: string }>;
      });
    },
  },

  users: {
    getProfile: (username: string) =>
      request<import('../types').User>(`/users/${username}/profile`),
    updateProfile: (data: { display_name?: string; bio?: string }) =>
      request<import('../types').User>('/users/me/profile', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
  },
};
