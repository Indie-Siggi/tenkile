import { useState, useEffect } from 'preact/hooks';
import { route } from 'preact-router';
import { fetchLibraries } from '../hooks/useApi.js';
import { useAuth } from '../hooks/useAuth.js';

export default function Dashboard() {
  const { user, logout } = useAuth();
  const [libraries, setLibraries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    loadLibraries();
  }, []);

  const loadLibraries = async () => {
    try {
      setLoading(true);
      const data = await fetchLibraries();
      setLibraries(data.libraries || []);
      setError(null);
    } catch (err) {
      console.error('Failed to load libraries:', err);
      setError('Unable to load libraries. Please try again later.');
    } finally {
      setLoading(false);
    }
  };

  const handleLibraryClick = (libraryId) => {
    route(`/library/${libraryId}`);
  };

  const handleLogout = async () => {
    await logout();
    route('/login');
  };

  if (loading) {
    return (
      <div class="dashboard loading">
        <div class="spinner"></div>
        <p>Loading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div class="dashboard error">
        <div class="error-card">
          <h2>Error</h2>
          <p>{error}</p>
          <button class="btn btn-primary" onClick={loadLibraries}>
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div class="dashboard">
      <div class="dashboard-header">
        <h1>Welcome{user?.username ? `, ${user.username}` : ''}</h1>
        <button class="btn btn-secondary" onClick={handleLogout}>
          Logout
        </button>
      </div>

      <section class="libraries-section">
        <h2>Your Libraries</h2>
        {libraries.length === 0 ? (
          <div class="empty-state">
            <p>No libraries configured yet.</p>
          </div>
        ) : (
          <div class="library-grid">
            {libraries.map((lib) => (
              <div
                key={lib.id}
                class="library-card"
                onClick={() => handleLibraryClick(lib.id)}
              >
                <div class="library-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                  </svg>
                </div>
                <div class="library-info">
                  <h3>{lib.name}</h3>
                  <span class="library-type">{lib.library_type || 'Media'}</span>
                  <span class="library-count">{lib.item_count || 0} items</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
