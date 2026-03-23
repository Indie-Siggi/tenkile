import { useState, useEffect } from 'preact/hooks';
import { route } from 'preact-router';
import { fetchLibrary, fetchLibraryItems } from '../hooks/useApi.js';

export default function Library({ id }) {
  const [library, setLibrary] = useState(null);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [offset, setOffset] = useState(0);
  const [total, setTotal] = useState(0);
  const limit = 50;

  useEffect(() => {
    if (id) {
      loadLibrary();
      loadItems();
    }
  }, [id, offset]);

  const loadLibrary = async () => {
    try {
      const data = await fetchLibrary(id);
      setLibrary(data);
    } catch (err) {
      console.error('Failed to load library:', err);
    }
  };

  const loadItems = async () => {
    try {
      setLoading(true);
      const data = await fetchLibraryItems(id, { offset, limit });
      setItems(data.items || []);
      setTotal(data.total || 0);
      setError(null);
    } catch (err) {
      setError(err.message || 'Failed to load items');
    } finally {
      setLoading(false);
    }
  };

  const handlePlay = (itemId) => {
    route(`/play/${itemId}`);
  };

  const handleBack = () => {
    route('/');
  };

  const formatDuration = (ms) => {
    if (!ms) return '';
    const hours = Math.floor(ms / 3600000);
    const minutes = Math.floor((ms % 3600000) / 60000);
    if (hours > 0) {
      return `${hours}h ${minutes}m`;
    }
    return `${minutes}m`;
  };

  const formatYear = (year) => {
    return year || '';
  };

  if (loading && items.length === 0) {
    return (
      <div class="library loading">
        <div class="spinner"></div>
        <p>Loading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div class="library error">
        <div class="error-card">
          <h2>Error</h2>
          <p>{error}</p>
          <button class="btn btn-primary" onClick={loadItems}>
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div class="library">
      <button class="btn btn-back" onClick={handleBack}>
        &larr; Back to Dashboard
      </button>

      <div class="library-header">
        <h1>{library?.name || 'Library'}</h1>
        <span class="item-count">{total} items</span>
      </div>

      {items.length === 0 ? (
        <div class="empty-state">
          <p>No items in this library yet.</p>
        </div>
      ) : (
        <>
          <div class="items-grid">
            {items.map((item) => (
              <div key={item.id} class="item-card">
                <div
                  class="item-poster"
                  onClick={() => handlePlay(item.id)}
                  style={{
                    backgroundImage: item.poster_url
                      ? `url(${item.poster_url})`
                      : 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                  }}
                >
                  {!item.poster_url && (
                    <span class="item-icon">🎬</span>
                  )}
                  <div class="item-overlay">
                    <span class="play-icon">▶</span>
                  </div>
                </div>
                <div class="item-info">
                  <h3 class="item-title">{item.title}</h3>
                  <div class="item-meta">
                    {item.year && <span>{item.year}</span>}
                    {item.duration_ms && (
                      <span>{formatDuration(item.duration_ms)}</span>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>

          {total > limit && (
            <div class="pagination">
              <button
                class="btn btn-secondary"
                onClick={() => setOffset(Math.max(0, offset - limit))}
                disabled={offset === 0}
              >
                Previous
              </button>
              <span class="page-info">
                {offset + 1}-{Math.min(offset + limit, total)} of {total}
              </span>
              <button
                class="btn btn-secondary"
                onClick={() => setOffset(offset + limit)}
                disabled={offset + limit >= total}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
