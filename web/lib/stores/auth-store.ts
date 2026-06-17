import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, LoginResponse, Organization } from '@/types/api';
import { setTokens, clearTokens, getAccessToken, getRefreshToken } from '@/lib/api/client';
import { authApi } from '@/lib/api/auth';

interface AuthState {
  user: User | null;
  organization: Organization | null;
  accessToken: string | null;
  isAuthenticated: boolean;

  // Actions
  login: (response: LoginResponse) => void;
  logout: () => Promise<void>;
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
        if (!response.access_token || !response.refresh_token || !response.user) {
          // Caller forgot to handle the require_totp branch. Refuse to
          // half-authenticate.
          throw new Error('login response missing tokens')
        }
        setTokens(response.access_token, response.refresh_token);
        set({
          user: response.user,
          organization: response.organization ?? null,
          accessToken: response.access_token,
          isAuthenticated: true,
        });
      },

      logout: async () => {
        const refreshToken = getRefreshToken();
        if (refreshToken) {
          // Tell backend to revoke the entire refresh token family.
          // Fire-and-forget — UX shouldn't block on this.
          authApi.logout(refreshToken).catch(() => {});
        }
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
