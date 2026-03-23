import { signal } from '@preact/signals';

const API_BASE = '/api/v1';

// Auth state signals
export const authState = {
  token: signal(localStorage.getItem('token') || null),
  user: signal(JSON.parse(localStorage.getItem('user') || 'null')),
  isAuthenticated: signal(!!localStorage.getItem('token')),
};

export function loadAuthFromStorage() {
  const token = localStorage.getItem('token');
  const user = JSON.parse(localStorage.getItem('user') || 'null');
  
  authState.token.value = token;
  authState.user.value = user;
  authState.isAuthenticated.value = !!token;
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
  
  // Store auth data
  localStorage.setItem('token', data.access_token);
  localStorage.setItem('refreshToken', data.refresh_token);
  localStorage.setItem('user', JSON.stringify(data.user));

  // Update signals
  authState.token.value = data.access_token;
  authState.user.value = data.user;
  authState.isAuthenticated.value = true;

  return data;
}

export async function logout() {
  try {
    await fetch(`${API_BASE}/auth/logout`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${authState.token.value}`,
      },
    });
  } catch (e) {
    // Ignore logout errors
  }

  // Clear stored auth
  localStorage.removeItem('token');
  localStorage.removeItem('refreshToken');
  localStorage.removeItem('user');

  // Update signals
  authState.token.value = null;
  authState.user.value = null;
  authState.isAuthenticated.value = false;
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
  const refreshToken = localStorage.getItem('refreshToken');
  if (!refreshToken) {
    throw new Error('No refresh token');
  }

  const response = await fetch(`${API_BASE}/auth/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });

  if (!response.ok) {
    logout();
    throw new Error('Token refresh failed');
  }

  const data = await response.json();
  localStorage.setItem('token', data.access_token);
  authState.token.value = data.access_token;

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
