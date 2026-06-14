import ky, { type KyInstance } from 'ky';

// token storage helpers (localStorage keys)
const ACCESS_TOKEN_KEY = 'ims_access_token';
const REFRESH_TOKEN_KEY = 'ims_refresh_token';

export const getAccessToken = () => {
  if (typeof window !== 'undefined') {
    return localStorage.getItem(ACCESS_TOKEN_KEY);
  }
  return null;
};

export const getRefreshToken = () => {
  if (typeof window !== 'undefined') {
    return localStorage.getItem(REFRESH_TOKEN_KEY);
  }
  return null;
};

export const setTokens = (access: string, refresh: string) => {
  if (typeof globalThis !== 'undefined') {
    localStorage.setItem(ACCESS_TOKEN_KEY, access);
    localStorage.setItem(REFRESH_TOKEN_KEY, refresh);
    document.cookie = `${ACCESS_TOKEN_KEY}=${access}; path=/; max-age=604800; SameSite=Lax; Secure`;
  }
};

export const clearTokens = () => {
  if (typeof globalThis !== 'undefined') {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    document.cookie = `${ACCESS_TOKEN_KEY}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; Secure`;
  }
};

let isRefreshing = false;

// ky instance
export const api: KyInstance = ky.create({
  prefix: process.env.NEXT_PUBLIC_API_URL,
  hooks: {
    beforeRequest: [
      ({ request }) => {
        const token = getAccessToken();
        if (token && !request.headers.has('Authorization')) {
          request.headers.set('Authorization', `Bearer ${token}`);
        }
      },
    ],
    afterResponse: [
      async ({ request, response }) => {
        if (response.status === 401) {
          if (isRefreshing) {
            clearTokens();
            if (typeof window !== 'undefined') {
              window.location.href = '/login';
            }
            return;
          }

          isRefreshing = true;

          const refreshToken = getRefreshToken();
          if (!refreshToken) {
            clearTokens();
            isRefreshing = false;
            if (typeof window !== 'undefined') {
              window.location.href = '/login';
            }
            return;
          }

          try {
            // Refresh call must use plain ky (not api) to avoid infinite loop
            const res = await ky.post('auth/refresh', {
              prefix: process.env.NEXT_PUBLIC_API_URL,
              json: { refresh_token: refreshToken },
            }).json<{ access_token: string; refresh_token: string }>();

            setTokens(res.access_token, res.refresh_token);

            // Retry the original request with the new access token
            request.headers.set('Authorization', `Bearer ${res.access_token}`);
            return await api(request);
          } catch (error) {
            clearTokens();
            if (typeof window !== 'undefined') {
              window.location.href = '/login';
            }
            throw error;
          } finally {
            isRefreshing = false;
          }
        }
      },
    ],
  },
});
