import { signal, effect } from '@preact/signals';

const API_BASE = '/api/v1';

// Auth state signals — single source of truth (in-memory).
// sessionStorage is used as fallback persistence (cleared on tab close).
export const authState = {
  token: signal(null),
  refreshTokenValue: signal(null),
  user: signal(null),
  isAuthenticated: signal(false),
};

// Load from sessionStorage only once at init
function _initFromStorage() {
  try {
    const token = sessionStorage.getItem('token');
    const refresh = sessionStorage.getItem('refreshToken');
    const user = JSON.parse(sessionStorage.getItem('user') || 'null');
    authState.token.value = token;
    authState.refreshTokenValue.value = refresh;
    authState.user.value = user;
    authState.isAuthenticated.value = !!token;
  } catch {
    // Storage unavailable or corrupt — start unauthenticated
    clearAuthState();
  }
}

// Sync signals TO sessionStorage whenever they change
effect(() => {
  const token = authState.token.value;
  const refresh = authState.refreshTokenValue.value;
  const user = authState.user.value;
  try {
    if (token) {
      sessionStorage.setItem('token', token);
    } else {
      sessionStorage.removeItem('token');
    }
    if (refresh) {
      sessionStorage.setItem('refreshToken', refresh);
    } else {
      sessionStorage.removeItem('refreshToken');
    }
    if (user) {
      sessionStorage.setItem('user', JSON.stringify(user));
    } else {
      sessionStorage.removeItem('user');
    }
  } catch {
    // Storage unavailable — tokens remain in memory only
  }
});

function clearAuthState() {
  authState.token.value = null;
  authState.refreshTokenValue.value = null;
  authState.user.value = null;
  authState.isAuthenticated.value = false;
}

// Migrate any tokens from localStorage (one-time cleanup) then init
export function loadAuthFromStorage() {
  try {
    // Migrate old localStorage tokens to sessionStorage, then remove
    const oldToken = localStorage.getItem('token');
    if (oldToken) {
      sessionStorage.setItem('token', oldToken);
      sessionStorage.setItem('refreshToken', localStorage.getItem('refreshToken') || '');
      const oldUser = localStorage.getItem('user');
      if (oldUser) sessionStorage.setItem('user', oldUser);
      localStorage.removeItem('token');
      localStorage.removeItem('refreshToken');
      localStorage.removeItem('user');
    }
  } catch {
    // Ignore migration errors
  }
  _initFromStorage();
}

export async function login(username, password) {
  const response = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ username, password }),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.message || 'Login failed');
  }

  const data = await response.json();

  // Update signals (effect syncs to sessionStorage automatically)
  authState.token.value = data.access_token;
  authState.refreshTokenValue.value = data.refresh_token;
  authState.user.value = data.user;
  authState.isAuthenticated.value = true;

  return data;
}

export async function logout() {
  const token = authState.token.value;
  try {
    const response = await fetch(`${API_BASE}/auth/logout`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });
    if (!response.ok) {
      console.warn('Server logout failed (status ' + response.status + '). Local session cleared.');
    }
  } catch (e) {
    console.warn('Server logout request failed. Local session cleared.');
  }

  // Always clear local state so user is not stuck logged in
  clearAuthState();
}

export async function getCurrentUser() {
  const response = await fetch(`${API_BASE}/auth/me`, {
    headers: {
      'Authorization': `Bearer ${authState.token.value}`,
    },
  });

  if (!response.ok) {
    throw new Error('Failed to get current user');
  }

  return response.json();
}

export async function refreshToken() {
  const refresh = authState.refreshTokenValue.value;
  if (!refresh) {
    throw new Error('No refresh token');
  }

  const response = await fetch(`${API_BASE}/auth/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ refresh_token: refresh }),
  });

  if (!response.ok) {
    clearAuthState();
    throw new Error('Token refresh failed');
  }

  const data = await response.json();
  authState.token.value = data.access_token;
  if (data.refresh_token) {
    authState.refreshTokenValue.value = data.refresh_token;
  }

  return data.access_token;
}

// useAuth hook
export function useAuth() {
  return {
    user: authState.user.value,
    isAuthenticated: authState.isAuthenticated.value,
    login,
    logout,
    getCurrentUser,
    refreshToken,
  };
}
