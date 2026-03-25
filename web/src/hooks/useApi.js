import { authState, refreshToken as doRefreshToken } from './useAuth.js';

const API_BASE = '/api/v1';

class ApiError extends Error {
  constructor(message, status, data) {
    super(message);
    this.status = status;
    this.data = data;
  }
}

// Guard to prevent concurrent refresh attempts
let _refreshPromise = null;

async function _performRequest(url, options, headers) {
  const response = await fetch(url, { ...options, headers });

  // Parse response
  const contentType = response.headers.get('content-type');
  let data;
  if (contentType && contentType.includes('application/json')) {
    data = await response.json();
  } else {
    data = await response.text();
  }

  if (!response.ok) {
    const message = data?.message || data?.error || 'Request failed';
    throw new ApiError(message, response.status, data);
  }

  return data;
}

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;

  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };

  // Add auth token if available
  const token = authState.token.value;
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  try {
    return await _performRequest(url, options, headers);
  } catch (err) {
    // On 401, attempt a single token refresh and retry
    if (err instanceof ApiError && err.status === 401 && authState.token.value) {
      try {
        // Coalesce concurrent refresh attempts
        if (!_refreshPromise) {
          _refreshPromise = doRefreshToken().finally(() => { _refreshPromise = null; });
        }
        const newToken = await _refreshPromise;

        // Retry with fresh token
        const retryHeaders = { ...headers, 'Authorization': `Bearer ${newToken}` };
        return await _performRequest(url, options, retryHeaders);
      } catch {
        // Refresh failed — redirect to login
        window.location.href = '/login';
        throw new ApiError('Session expired. Please log in again.', 401, null);
      }
    }
    throw err;
  }
}

// GET request
export async function get(endpoint, params = {}) {
  const queryString = new URLSearchParams(params).toString();
  const url = queryString ? `${endpoint}?${queryString}` : endpoint;
  return request(url, { method: 'GET' });
}

// POST request
export async function post(endpoint, body = {}) {
  return request(endpoint, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

// PUT request
export async function put(endpoint, body = {}) {
  return request(endpoint, {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

// PATCH request
export async function patch(endpoint, body = {}) {
  return request(endpoint, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

// DELETE request
export async function del(endpoint) {
  return request(endpoint, { method: 'DELETE' });
}

// API-specific functions
export async function fetchLibraries() {
  return get('/libraries');
}

export async function fetchLibrary(id) {
  return get(`/libraries/${id}`);
}

export async function fetchLibraryItems(libraryId, params = {}) {
  return get(`/libraries/${libraryId}/items`, params);
}

export async function fetchMediaItem(id) {
  return get(`/media/${id}`);
}

export async function fetchMediaStreamInfo(id) {
  return get(`/media/${id}/play`);
}

export async function fetchHLSManifest(mediaId, variant) {
  const params = variant ? `?variant=${variant}` : '';
  return get(`/stream/hls/${mediaId}${params}`);
}

// useApi hook
export function useApi() {
  return {
    get,
    post,
    put,
    patch,
    del,
    fetchLibraries,
    fetchLibrary,
    fetchLibraryItems,
    fetchMediaItem,
    fetchMediaStreamInfo,
    fetchHLSManifest,
    ApiError,
  };
}
