import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, LoginResponse, Organization } from '@/types/api';
import { setTokens, clearTokens, getAccessToken } from '@/lib/api/client';

interface AuthState {
  user: User | null;
  organization: Organization | null;
  accessToken: string | null;
  isAuthenticated: boolean;

  // Actions
  login: (response: LoginResponse) => void;
  logout: () => void;
  setUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      organization: null,
      accessToken: null,
      isAuthenticated: false,

      login: (response) => {
        setTokens(response.access_token, response.refresh_token);
        set({
          user: response.user,
          organization: response.organization,
          accessToken: response.access_token,
          isAuthenticated: true,
        });
      },

      logout: () => {
        clearTokens();
        set({
          user: null,
          organization: null,
          accessToken: null,
          isAuthenticated: false,
        });
        if (typeof window !== 'undefined') {
          window.location.href = '/login';
        }
      },

      setUser: (user) => {
        set({ user });
      },
    }),
    {
      name: 'ims_auth',
      // Persist only the user and org state, not tokens (tokens are handled separately in client.ts localStorage helpers)
      partialize: (state) => ({
        user: state.user,
        organization: state.organization,
      }),
      // Rehydrate client token and authenticated state on store loading/hydration
      onRehydrateStorage: () => (state) => {
        if (state) {
          const token = getAccessToken();
          state.accessToken = token;
          state.isAuthenticated = !!token;
        }
      },
    }
  )
);
