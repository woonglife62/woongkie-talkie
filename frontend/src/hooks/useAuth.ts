import { useEffect } from 'react';
import { useAuthStore } from '../stores/authStore';

export function useAuth() {
  const { user, isAuthenticated, isLoading, fetchMe, login, register, logout } = useAuthStore();

  useEffect(() => {
    if (!user) {
      fetchMe();
    }
  }, []);

  return { user, isAuthenticated, isLoading, login, register, logout };
}
